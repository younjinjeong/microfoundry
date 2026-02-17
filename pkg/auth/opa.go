package auth

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sync"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/rego"
)

//go:embed policies/*.rego
var defaultPolicies embed.FS

// AuthzInput is the input document sent to OPA for authorization decisions.
type AuthzInput struct {
	User     *AuthzUser `json:"user"`
	Action   string     `json:"action"`
	Resource string     `json:"resource"`
	OrgID    string     `json:"org_id,omitempty"`
	Path     string     `json:"path"`
	Method   string     `json:"method"`
}

// AuthzUser represents the user context for OPA evaluation.
type AuthzUser struct {
	ID      string   `json:"id"`
	Email   string   `json:"email"`
	Roles   []string `json:"roles"`
	OrgRole string   `json:"org_role"`
}

// AuthzResult contains the OPA decision.
type AuthzResult struct {
	Allow  bool   `json:"allow"`
	Reason string `json:"reason,omitempty"`
}

// OPAEngine evaluates authorization decisions using compiled Rego policies.
type OPAEngine struct {
	mu       sync.RWMutex
	modules  map[string]string // name → rego source
	compiler *ast.Compiler
	query    string
}

// NewOPAEngine creates an OPA engine with the embedded default policies.
func NewOPAEngine() (*OPAEngine, error) {
	engine := &OPAEngine{
		modules: make(map[string]string),
		query:   "data.microfoundry.authz.allow",
	}

	// Load embedded policies
	err := fs.WalkDir(defaultPolicies, "policies", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := defaultPolicies.ReadFile(path)
		if err != nil {
			return err
		}
		engine.modules[path] = string(data)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("loading embedded policies: %w", err)
	}

	if err := engine.compile(); err != nil {
		return nil, err
	}

	return engine, nil
}

// compile recompiles all loaded modules.
func (e *OPAEngine) compile() error {
	compiler, err := ast.CompileModulesWithOpt(e.modules, ast.CompileOpts{
		EnablePrintStatements: false,
	})
	if err != nil {
		return fmt.Errorf("compiling rego policies: %w", err)
	}
	e.compiler = compiler
	return nil
}

// Evaluate runs the authorization query against the compiled policies.
func (e *OPAEngine) Evaluate(ctx context.Context, input AuthzInput) (AuthzResult, error) {
	e.mu.RLock()
	compiler := e.compiler
	e.mu.RUnlock()

	r := rego.New(
		rego.Query(e.query),
		rego.Compiler(compiler),
		rego.Input(map[string]any{
			"user":     toMap(input.User),
			"action":   input.Action,
			"resource": input.Resource,
			"org_id":   input.OrgID,
			"path":     input.Path,
			"method":   input.Method,
		}),
	)

	rs, err := r.Eval(ctx)
	if err != nil {
		return AuthzResult{}, fmt.Errorf("evaluating policy: %w", err)
	}

	if len(rs) == 0 || len(rs[0].Expressions) == 0 {
		return AuthzResult{Allow: false, Reason: "no policy result"}, nil
	}

	allowed, ok := rs[0].Expressions[0].Value.(bool)
	if !ok {
		return AuthzResult{Allow: false, Reason: "invalid policy result type"}, nil
	}

	result := AuthzResult{Allow: allowed}
	if !allowed {
		result.Reason = fmt.Sprintf("denied: %s %s on %s", input.Method, input.Action, input.Resource)
	}
	return result, nil
}

// UpdatePolicy adds or replaces a custom policy module and recompiles.
// Uses copy-on-write to avoid corrupting modules map if compilation fails.
func (e *OPAEngine) UpdatePolicy(name, regoText string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Copy modules to avoid mutation before successful compile
	updated := make(map[string]string, len(e.modules)+1)
	for k, v := range e.modules {
		updated[k] = v
	}
	updated[name] = regoText

	compiler, err := ast.CompileModulesWithOpt(updated, ast.CompileOpts{
		EnablePrintStatements: false,
	})
	if err != nil {
		return fmt.Errorf("compiling rego policies: %w", err)
	}

	// Swap atomically only after successful compile
	e.modules = updated
	e.compiler = compiler
	return nil
}

// GetPolicies returns the currently loaded policy names and their source.
func (e *OPAEngine) GetPolicies() map[string]string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make(map[string]string, len(e.modules))
	for k, v := range e.modules {
		result[k] = v
	}
	return result
}

// toMap converts an AuthzUser to a map for OPA input, or nil if user is nil.
func toMap(user *AuthzUser) any {
	if user == nil {
		return nil
	}
	return map[string]any{
		"id":       user.ID,
		"email":    user.Email,
		"roles":    user.Roles,
		"org_role": user.OrgRole,
	}
}
