package terraform

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/hashicorp/terraform-exec/tfexec"
	"github.com/younjinjeong/microfoundry/pkg/k8s"
	"github.com/younjinjeong/microfoundry/pkg/models"
)

// Executor runs terraform init/plan/apply/destroy for service topologies.
type Executor struct {
	execPath      string
	topologyStore *TopologyStore
	stateStore    *StateStore
	k8sClient     *k8s.Client
	mu            sync.Mutex
}

// NewExecutor creates a new Terraform executor. Returns an error if terraform is not on PATH.
func NewExecutor(client *k8s.Client) (*Executor, error) {
	execPath, err := exec.LookPath("terraform")
	if err != nil {
		return nil, fmt.Errorf("terraform binary not found: %w", err)
	}
	return &Executor{
		execPath:      execPath,
		topologyStore: NewTopologyStore(client),
		stateStore:    NewStateStore(client),
		k8sClient:     client,
	}, nil
}

// IsAvailable returns true if terraform binary is on PATH.
func IsAvailable() bool {
	_, err := exec.LookPath("terraform")
	return err == nil
}

// TopologyStore returns the underlying topology store.
func (e *Executor) TopologyStore() *TopologyStore {
	return e.topologyStore
}

// HasTopology checks if a topology config exists for a service-type + plan.
func (e *Executor) HasTopology(ctx context.Context, serviceType, planID string) bool {
	return e.topologyStore.Exists(ctx, serviceType, planID)
}

// Provision runs terraform init + apply and returns the credential map.
func (e *Executor) Provision(ctx context.Context, instanceName, serviceTypeID, planID, password string, plan models.ServicePlan) (map[string]string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	topo, err := e.topologyStore.Get(ctx, serviceTypeID, planID)
	if err != nil {
		return nil, fmt.Errorf("loading topology: %w", err)
	}

	workDir, err := os.MkdirTemp("", "mf-tf-"+instanceName+"-*")
	if err != nil {
		return nil, fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(workDir)

	// Write .tf files
	for name, content := range topo.Files {
		if err := os.WriteFile(filepath.Join(workDir, name), []byte(content), 0644); err != nil {
			return nil, fmt.Errorf("writing %s: %w", name, err)
		}
	}

	// Write terraform.tfvars
	k8sName := models.ServiceSecretPrefix + instanceName
	tfvars := fmt.Sprintf(`instance_name   = %q
namespace       = %q
password        = %q
k8s_name        = %q
plan_memory_mb  = %d
plan_cpu_millis = %d
plan_storage_gb = %d
`, instanceName, e.k8sClient.Namespace, password, k8sName,
		plan.Resources.MemoryMB, plan.Resources.CPUMillis, plan.Resources.StorageGB)

	// Inject AWS-specific vars when running on AWS (Fargate / EC2)
	if region := os.Getenv("AWS_DEFAULT_REGION"); region != "" {
		tfvars += fmt.Sprintf("aws_region = %q\n", region)
	}
	if vpcID := os.Getenv("MF_VPC_ID"); vpcID != "" {
		tfvars += fmt.Sprintf("vpc_id = %q\n", vpcID)
	}
	if subnetIDs := os.Getenv("MF_SUBNET_IDS"); subnetIDs != "" {
		tfvars += fmt.Sprintf("subnet_ids = %s\n", subnetIDs)
	}

	if err := os.WriteFile(filepath.Join(workDir, "terraform.tfvars"), []byte(tfvars), 0644); err != nil {
		return nil, fmt.Errorf("writing terraform.tfvars: %w", err)
	}

	// Load existing state if re-running
	existingState, _ := e.stateStore.Get(ctx, instanceName)
	if existingState != "" {
		if err := os.WriteFile(filepath.Join(workDir, "terraform.tfstate"), []byte(existingState), 0644); err != nil {
			return nil, fmt.Errorf("writing existing state: %w", err)
		}
	}

	// Create terraform instance
	tf, err := tfexec.NewTerraform(workDir, e.execPath)
	if err != nil {
		return nil, fmt.Errorf("initializing terraform: %w", err)
	}

	var stderr bytes.Buffer
	tf.SetStderr(&stderr)

	// Init
	if err := tf.Init(ctx); err != nil {
		return nil, fmt.Errorf("terraform init: %w\nstderr: %s", err, stderr.String())
	}

	// Apply
	stderr.Reset()
	if err := tf.Apply(ctx); err != nil {
		// Save state even on error for partial apply recovery
		e.saveStateFromDir(ctx, workDir, instanceName, serviceTypeID, planID)
		return nil, fmt.Errorf("terraform apply: %w\nstderr: %s", err, stderr.String())
	}

	// Save state
	if err := e.saveStateFromDir(ctx, workDir, instanceName, serviceTypeID, planID); err != nil {
		log.Printf("warning: failed to save terraform state for %q: %v", instanceName, err)
	}

	// Extract outputs
	outputs, err := tf.Output(ctx)
	if err != nil {
		return nil, fmt.Errorf("terraform output: %w", err)
	}

	creds := make(map[string]string)
	for k, v := range outputs {
		var val string
		if err := json.Unmarshal(v.Value, &val); err != nil {
			// Try as raw string
			val = string(v.Value)
		}
		creds[k] = val
	}

	return creds, nil
}

// Destroy runs terraform init + destroy and cleans up state.
func (e *Executor) Destroy(ctx context.Context, instanceName string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Load state
	stateJSON, err := e.stateStore.Get(ctx, instanceName)
	if err != nil {
		return fmt.Errorf("loading terraform state: %w", err)
	}

	// Load topology key from state labels
	topoKey, err := e.stateStore.GetTopologyKey(ctx, instanceName)
	if err != nil {
		return fmt.Errorf("loading topology key: %w", err)
	}

	// Load .tf files from topology
	topo, err := e.topologyStore.Get(ctx, topoKey.ServiceType, topoKey.PlanID)
	if err != nil {
		return fmt.Errorf("loading topology for destroy: %w", err)
	}

	workDir, err := os.MkdirTemp("", "mf-tf-destroy-"+instanceName+"-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(workDir)

	// Write .tf files
	for name, content := range topo.Files {
		if err := os.WriteFile(filepath.Join(workDir, name), []byte(content), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", name, err)
		}
	}

	// Write state file
	if err := os.WriteFile(filepath.Join(workDir, "terraform.tfstate"), []byte(stateJSON), 0644); err != nil {
		return fmt.Errorf("writing state file: %w", err)
	}

	// Write tfvars with placeholder values (variables still needed for destroy)
	k8sName := models.ServiceSecretPrefix + instanceName
	tfvars := fmt.Sprintf(`instance_name   = %q
namespace       = %q
password        = "destroy-placeholder"
k8s_name        = %q
plan_memory_mb  = 0
plan_cpu_millis = 0
plan_storage_gb = 0
`, instanceName, e.k8sClient.Namespace, k8sName)

	if err := os.WriteFile(filepath.Join(workDir, "terraform.tfvars"), []byte(tfvars), 0644); err != nil {
		return fmt.Errorf("writing terraform.tfvars: %w", err)
	}

	tf, err := tfexec.NewTerraform(workDir, e.execPath)
	if err != nil {
		return fmt.Errorf("initializing terraform: %w", err)
	}

	var stderr bytes.Buffer
	tf.SetStderr(&stderr)

	// Init
	if err := tf.Init(ctx); err != nil {
		return fmt.Errorf("terraform init: %w\nstderr: %s", err, stderr.String())
	}

	// Destroy
	stderr.Reset()
	if err := tf.Destroy(ctx); err != nil {
		return fmt.Errorf("terraform destroy: %w\nstderr: %s", err, stderr.String())
	}

	// Delete state ConfigMap
	return e.stateStore.Delete(ctx, instanceName)
}

// Plan runs terraform init + plan for admin preview.
func (e *Executor) Plan(ctx context.Context, serviceTypeID, planID string, plan models.ServicePlan) (*models.TopologyPlanResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	topo, err := e.topologyStore.Get(ctx, serviceTypeID, planID)
	if err != nil {
		return &models.TopologyPlanResult{
			ServiceType: serviceTypeID,
			PlanID:      planID,
			Error:       fmt.Sprintf("loading topology: %v", err),
		}, nil
	}

	workDir, err := os.MkdirTemp("", "mf-tf-plan-*")
	if err != nil {
		return nil, fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(workDir)

	// Write .tf files
	for name, content := range topo.Files {
		if err := os.WriteFile(filepath.Join(workDir, name), []byte(content), 0644); err != nil {
			return nil, fmt.Errorf("writing %s: %w", name, err)
		}
	}

	// Write sample terraform.tfvars
	tfvars := fmt.Sprintf(`instance_name   = "preview-instance"
namespace       = %q
password        = "preview-password-placeholder"
k8s_name        = "mf-svc-preview-instance"
plan_memory_mb  = %d
plan_cpu_millis = %d
plan_storage_gb = %d
`, e.k8sClient.Namespace, plan.Resources.MemoryMB, plan.Resources.CPUMillis, plan.Resources.StorageGB)

	if err := os.WriteFile(filepath.Join(workDir, "terraform.tfvars"), []byte(tfvars), 0644); err != nil {
		return nil, fmt.Errorf("writing terraform.tfvars: %w", err)
	}

	tf, err := tfexec.NewTerraform(workDir, e.execPath)
	if err != nil {
		return nil, fmt.Errorf("initializing terraform: %w", err)
	}

	var stdout, stderr bytes.Buffer
	tf.SetStdout(&stdout)
	tf.SetStderr(&stderr)

	// Init
	if err := tf.Init(ctx); err != nil {
		return &models.TopologyPlanResult{
			ServiceType: serviceTypeID,
			PlanID:      planID,
			Error:       fmt.Sprintf("terraform init failed:\n%s\n%s", err, stderr.String()),
		}, nil
	}

	// Plan
	stdout.Reset()
	stderr.Reset()
	hasChanges, err := tf.Plan(ctx)
	if err != nil {
		return &models.TopologyPlanResult{
			ServiceType: serviceTypeID,
			PlanID:      planID,
			Error:       fmt.Sprintf("terraform plan failed:\n%s\n%s", err, stderr.String()),
		}, nil
	}

	output := stdout.String()
	if output == "" {
		output = stderr.String()
	}
	if output == "" {
		if hasChanges {
			output = "Plan shows changes would be made."
		} else {
			output = "No changes. Infrastructure is up-to-date."
		}
	}

	return &models.TopologyPlanResult{
		ServiceType: serviceTypeID,
		PlanID:      planID,
		Output:      output,
		HasChanges:  hasChanges,
	}, nil
}

func (e *Executor) saveStateFromDir(ctx context.Context, workDir, instanceName, serviceType, planID string) error {
	stateBytes, err := os.ReadFile(filepath.Join(workDir, "terraform.tfstate"))
	if err != nil {
		return fmt.Errorf("reading state file: %w", err)
	}
	return e.stateStore.SaveWithTopologyKey(ctx, instanceName, string(stateBytes), serviceType, planID)
}
