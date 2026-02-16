package admin

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/younjinjeong/microfoundry/pkg/models"
	"github.com/younjinjeong/microfoundry/pkg/service"
	"github.com/younjinjeong/microfoundry/pkg/terraform"
)

// TopologiesListHandler shows the topology management page.
func (s *Server) TopologiesListHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	client, err := s.getClient(r)
	if err != nil {
		http.Error(w, "No cluster available: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	store := terraform.NewTopologyStore(client)
	items, err := store.List(ctx, service.Catalog())
	if err != nil {
		log.Printf("error listing topologies: %v", err)
		items = []models.TopologyListItem{}
	}

	data := s.pageData("Service Topologies", "topologies")
	data.Content = map[string]any{
		"Items":              items,
		"TerraformAvailable": terraform.IsAvailable(),
	}
	s.templates.Render(w, "topologies.html", data)
}

// TopologyDetailHandler shows the edit/review page for a topology.
func (s *Server) TopologyDetailHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	serviceType := r.PathValue("type")
	planID := r.PathValue("plan")

	client, err := s.getClient(r)
	if err != nil {
		http.Error(w, "No cluster available: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	plan, ok := service.FindPlan(serviceType, planID)
	if !ok {
		http.Error(w, fmt.Sprintf("Plan %s/%s not found in catalog", serviceType, planID), http.StatusNotFound)
		return
	}

	store := terraform.NewTopologyStore(client)
	topo, _ := store.Get(ctx, serviceType, planID)

	content := map[string]any{
		"ServiceType": serviceType,
		"PlanID":      planID,
		"Plan":        plan,
		"HasTopology": topo != nil,
		"Version":     0,
		"Description": "",
		"Files":       map[string]string{},
	}

	if topo != nil {
		content["Version"] = topo.Metadata.Version
		content["Description"] = topo.Metadata.Description
		content["Files"] = topo.Files
	}

	data := s.pageData(serviceType+"/"+planID, "topologies")
	data.Content = content
	s.templates.Render(w, "topology_detail.html", data)
}

// SaveTopologyHandler saves topology files from the editor form.
func (s *Server) SaveTopologyHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	serviceType := r.PathValue("type")
	planID := r.PathValue("plan")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	client, err := s.getClient(r)
	if err != nil {
		http.Error(w, "No cluster available: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	fileNames := r.Form["file_name"]
	fileContents := r.Form["file_content"]
	description := r.FormValue("description")

	files := make(map[string]string)
	for i := range fileNames {
		name := strings.TrimSpace(fileNames[i])
		if name == "" || i >= len(fileContents) {
			continue
		}
		if !strings.HasSuffix(name, ".tf") {
			name += ".tf"
		}
		files[name] = fileContents[i]
	}

	if len(files) == 0 {
		http.Error(w, "No .tf files provided", http.StatusBadRequest)
		return
	}

	config := &models.TopologyConfig{
		ServiceType: serviceType,
		PlanID:      planID,
		Metadata: models.TopologyMetadata{
			Description: description,
		},
		Files: files,
	}

	store := terraform.NewTopologyStore(client)
	if err := store.Save(ctx, config); err != nil {
		http.Error(w, "Failed to save topology: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/topologies/"+serviceType+"/"+planID, http.StatusSeeOther)
}

// TopologyPreviewHandler runs terraform plan and returns the output.
func (s *Server) TopologyPreviewHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	serviceType := r.PathValue("type")
	planID := r.PathValue("plan")

	if !terraform.IsAvailable() {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<span class="text-yellow-400">Terraform is not installed on this system. Install terraform to use plan preview.</span>`)
		return
	}

	if err := r.ParseForm(); err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<span class="text-red-400">Invalid form data: %s</span>`, err)
		return
	}

	// First save the current form content so we can plan it
	client, err := s.getClient(r)
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<span class="text-red-400">No cluster available: %s</span>`, err)
		return
	}

	fileNames := r.Form["file_name"]
	fileContents := r.Form["file_content"]
	description := r.FormValue("description")

	files := make(map[string]string)
	for i := range fileNames {
		name := strings.TrimSpace(fileNames[i])
		if name == "" || i >= len(fileContents) {
			continue
		}
		if !strings.HasSuffix(name, ".tf") {
			name += ".tf"
		}
		files[name] = fileContents[i]
	}

	if len(files) == 0 {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<span class="text-red-400">No .tf files to plan</span>`)
		return
	}

	// Save temporarily
	config := &models.TopologyConfig{
		ServiceType: serviceType,
		PlanID:      planID,
		Metadata:    models.TopologyMetadata{Description: description},
		Files:       files,
	}
	store := terraform.NewTopologyStore(client)
	if err := store.Save(ctx, config); err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<span class="text-red-400">Failed to save before plan: %s</span>`, err)
		return
	}

	plan, ok := service.FindPlan(serviceType, planID)
	if !ok {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<span class="text-red-400">Plan %s/%s not found</span>`, serviceType, planID)
		return
	}

	tfExec, err := terraform.NewExecutor(client)
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<span class="text-red-400">Terraform executor error: %s</span>`, err)
		return
	}

	result, err := tfExec.Plan(ctx, serviceType, planID, plan)
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<span class="text-red-400">Plan error: %s</span>`, err)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if result.Error != "" {
		fmt.Fprintf(w, `<span class="text-red-400">%s</span>`, result.Error)
	} else if result.HasChanges {
		fmt.Fprintf(w, `<span class="text-yellow-300">Plan shows changes:</span>\n%s`, result.Output)
	} else {
		fmt.Fprintf(w, `<span class="text-green-400">%s</span>`, result.Output)
	}
}

// TopologyUploadHandler shows the upload form.
func (s *Server) TopologyUploadHandler(w http.ResponseWriter, r *http.Request) {
	data := s.pageData("Upload Topology", "topologies")
	data.Content = map[string]any{
		"ServiceTypes": service.Catalog(),
	}
	s.templates.Render(w, "topology_upload.html", data)
}

// UploadTopologyHandler handles multipart file upload.
func (s *Server) UploadTopologyHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Failed to parse upload: "+err.Error(), http.StatusBadRequest)
		return
	}

	serviceType := r.FormValue("service_type")
	planID := r.FormValue("plan_id")
	description := r.FormValue("description")

	if serviceType == "" || planID == "" {
		http.Error(w, "Service type and plan are required", http.StatusBadRequest)
		return
	}

	client, err := s.getClient(r)
	if err != nil {
		http.Error(w, "No cluster available: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	files := make(map[string]string)
	fhs := r.MultipartForm.File["tf_files"]
	for _, fh := range fhs {
		if !strings.HasSuffix(fh.Filename, ".tf") {
			continue
		}
		f, err := fh.Open()
		if err != nil {
			continue
		}
		content, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			continue
		}
		files[fh.Filename] = string(content)
	}

	if len(files) == 0 {
		http.Error(w, "No valid .tf files uploaded", http.StatusBadRequest)
		return
	}

	config := &models.TopologyConfig{
		ServiceType: serviceType,
		PlanID:      planID,
		Metadata: models.TopologyMetadata{
			Description: description,
		},
		Files: files,
	}

	store := terraform.NewTopologyStore(client)
	if err := store.Save(ctx, config); err != nil {
		http.Error(w, "Failed to save topology: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/topologies/"+serviceType+"/"+planID, http.StatusSeeOther)
}

// DeleteTopologyHandler removes a topology config.
func (s *Server) DeleteTopologyHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	serviceType := r.PathValue("type")
	planID := r.PathValue("plan")

	client, err := s.getClient(r)
	if err != nil {
		http.Error(w, "No cluster available: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	store := terraform.NewTopologyStore(client)
	if err := store.Delete(ctx, serviceType, planID); err != nil {
		http.Error(w, "Failed to delete topology: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// HTMX-compatible: redirect on delete
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/topologies")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/topologies", http.StatusSeeOther)
}

// --- JSON API Handlers ---

// APITopologiesListHandler returns topology list as JSON.
func (s *Server) APITopologiesListHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	client, err := s.getClient(r)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}

	store := terraform.NewTopologyStore(client)
	items, err := store.List(ctx, service.Catalog())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, items)
}

// APITopologyDetailHandler returns a topology config as JSON.
func (s *Server) APITopologyDetailHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	serviceType := r.PathValue("type")
	planID := r.PathValue("plan")

	client, err := s.getClient(r)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}

	store := terraform.NewTopologyStore(client)
	topo, err := store.Get(ctx, serviceType, planID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, topo)
}

// APISaveTopologyHandler saves a topology from JSON body.
func (s *Server) APISaveTopologyHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	serviceType := r.PathValue("type")
	planID := r.PathValue("plan")

	client, err := s.getClient(r)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}

	var config models.TopologyConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON: " + err.Error()})
		return
	}
	config.ServiceType = serviceType
	config.PlanID = planID

	store := terraform.NewTopologyStore(client)
	if err := store.Save(ctx, &config); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

