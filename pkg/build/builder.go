package build

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Builder handles source-to-image builds.
type Builder struct {
	// ImagePrefix is prepended to app names for image tags.
	// For local: "microfoundry/" or "localhost:5000/"
	ImagePrefix string
}

// NewBuilder creates a builder with the given image prefix.
func NewBuilder(imagePrefix string) *Builder {
	if imagePrefix == "" {
		imagePrefix = "microfoundry/"
	}
	return &Builder{ImagePrefix: imagePrefix}
}

// BuildResult contains the output of a build operation.
type BuildResult struct {
	ImageRef string
	Logs     string
}

// Build builds a container image from source using the best available strategy.
func (b *Builder) Build(appName, srcDir string) (*BuildResult, error) {
	srcDir, err := filepath.Abs(srcDir)
	if err != nil {
		return nil, fmt.Errorf("resolving source path: %w", err)
	}

	imageTag := fmt.Sprintf("%s%s:latest", b.ImagePrefix, appName)

	// Strategy 1: Dockerfile present -> docker build
	if _, err := os.Stat(filepath.Join(srcDir, "Dockerfile")); err == nil {
		return b.dockerBuild(imageTag, srcDir)
	}

	// Strategy 2: pack CLI available -> CNB build
	if _, err := exec.LookPath("pack"); err == nil {
		return b.packBuild(imageTag, srcDir)
	}

	// Strategy 3: Dockerfile not found, pack not available
	return nil, fmt.Errorf("no Dockerfile found and 'pack' CLI not installed; install pack CLI or add a Dockerfile")
}

// dockerBuild builds using docker build.
func (b *Builder) dockerBuild(imageTag, srcDir string) (*BuildResult, error) {
	cmd := exec.Command("docker", "build", "-t", imageTag, ".")
	cmd.Dir = srcDir

	output, err := cmd.CombinedOutput()
	logs := string(output)
	if err != nil {
		return nil, fmt.Errorf("docker build failed: %s\n%s", err, logs)
	}

	return &BuildResult{
		ImageRef: imageTag,
		Logs:     logs,
	}, nil
}

// packBuild builds using Cloud Native Buildpacks (pack CLI).
func (b *Builder) packBuild(imageTag, srcDir string) (*BuildResult, error) {
	args := []string{
		"build", imageTag,
		"--builder", "paketobuildpacks/builder-jammy-base",
		"--path", srcDir,
	}

	cmd := exec.Command("pack", args...)
	output, err := cmd.CombinedOutput()
	logs := string(output)
	if err != nil {
		return nil, fmt.Errorf("pack build failed: %s\n%s", err, logs)
	}

	return &BuildResult{
		ImageRef: imageTag,
		Logs:     logs,
	}, nil
}

// DetectBuildStrategy returns the build strategy that will be used.
func DetectBuildStrategy(srcDir string) string {
	if _, err := os.Stat(filepath.Join(srcDir, "Dockerfile")); err == nil {
		return "docker"
	}
	if _, err := exec.LookPath("pack"); err == nil {
		return "cnb"
	}
	return "none"
}

// Login authenticates with a Docker registry via stdin to avoid leaking
// credentials in the process list.
func (b *Builder) Login(registry, username, password string) error {
	cmd := exec.Command("docker", "login", registry,
		"--username", username,
		"--password-stdin")
	cmd.Stdin = strings.NewReader(password)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker login failed: %s\n%s", err, string(output))
	}
	return nil
}

// Push pushes a built image to the remote registry.
func (b *Builder) Push(imageRef string) error {
	cmd := exec.Command("docker", "push", imageRef)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker push failed: %s\n%s", err, string(output))
	}
	return nil
}

// ImageExists checks if a Docker image exists locally.
func ImageExists(imageRef string) bool {
	cmd := exec.Command("docker", "image", "inspect", imageRef)
	err := cmd.Run()
	return err == nil
}
