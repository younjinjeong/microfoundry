package admin

import (
	"fmt"
	"net/http"

	"github.com/younjinjeong/microfoundry/pkg/models"
	"github.com/younjinjeong/microfoundry/pkg/service"
)

// ServicesListHandler shows all provisioned service instances.
func (s *Server) ServicesListHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	client, err := s.getClient(r)
	if err != nil {
		http.Error(w, "No cluster available: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	mgr := service.NewManager(client)
	items, err := mgr.List(ctx)
	if err != nil {
		items = []models.ServiceListItem{} // empty on error
	}

	data := s.pageData("Backing Services", "services")
	data.Content = map[string]any{
		"Services": items,
	}
	s.templates.Render(w, "services.html", data)
}

// ServiceDetailHandler shows details of a single service instance.
func (s *Server) ServiceDetailHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := r.PathValue("name")

	client, err := s.getClient(r)
	if err != nil {
		http.Error(w, "No cluster available: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	mgr := service.NewManager(client)
	inst, err := mgr.Get(ctx, name)
	if err != nil {
		http.Error(w, "Service not found: "+err.Error(), http.StatusNotFound)
		return
	}

	// Find plan details from catalog
	plan, _ := service.FindPlan(inst.ServiceType, inst.Plan)

	data := s.pageData(name, "services")
	data.Content = map[string]any{
		"Instance": inst,
		"Plan":     plan,
	}
	s.templates.Render(w, "service_detail.html", data)
}

// MarketplaceHandler shows the service catalog.
func (s *Server) MarketplaceHandler(w http.ResponseWriter, r *http.Request) {
	catalog := service.Catalog()

	data := s.pageData("Service Marketplace", "marketplace")
	data.Content = map[string]any{
		"Catalog": catalog,
	}
	s.templates.Render(w, "marketplace.html", data)
}

// CreateServiceHandler provisions a new service instance from the marketplace.
func (s *Server) CreateServiceHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	serviceType := r.FormValue("service_type")
	plan := r.FormValue("plan")
	name := r.FormValue("name")

	if serviceType == "" || plan == "" || name == "" {
		http.Error(w, "service_type, plan, and name are required", http.StatusBadRequest)
		return
	}

	_, ok := service.FindServiceType(serviceType)
	if !ok {
		http.Error(w, fmt.Sprintf("unknown service type: %s", serviceType), http.StatusBadRequest)
		return
	}
	if _, ok := service.FindPlan(serviceType, plan); !ok {
		http.Error(w, fmt.Sprintf("unknown plan %q for service %q", plan, serviceType), http.StatusBadRequest)
		return
	}

	client, err := s.getClient(r)
	if err != nil {
		http.Error(w, "No cluster available: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	mgr := service.NewManager(client)

	// Check if already exists
	if existing, _ := mgr.Get(ctx, name); existing != nil {
		http.Error(w, fmt.Sprintf("service instance %q already exists", name), http.StatusConflict)
		return
	}

	inst := &models.ServiceInstance{
		Name:        name,
		ServiceType: serviceType,
		Plan:        plan,
		ClusterID:   s.clientManager.GetActive(),
	}

	if err := mgr.Create(ctx, inst); err != nil {
		http.Error(w, "creating service: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Mock outputs (will be replaced by Terraform in future)
	outputs := models.ServiceOutputs{
		Host:     fmt.Sprintf("%s.cluster.local", name),
		Port:     3306,
		Username: "admin",
		Password: "changeme",
		Database: name,
		URI:      fmt.Sprintf("mysql://admin:changeme@%s.cluster.local:3306/%s", name, name),
	}
	_ = mgr.SaveOutputs(ctx, name, outputs)
	_ = mgr.UpdateStatus(ctx, name, models.ServiceStatusAvailable, "")

	w.Header().Set("HX-Redirect", "/services/"+name)
	w.WriteHeader(http.StatusOK)
}

// BindServiceHandler binds a service instance to an app.
func (s *Server) BindServiceHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := r.PathValue("name")
	appName := r.FormValue("app_name")

	if appName == "" {
		http.Error(w, "app_name is required", http.StatusBadRequest)
		return
	}

	client, err := s.getClient(r)
	if err != nil {
		http.Error(w, "No cluster available: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	mgr := service.NewManager(client)
	binder := service.NewBinder(client)

	inst, err := mgr.Get(ctx, name)
	if err != nil {
		http.Error(w, "Service not found: "+err.Error(), http.StatusNotFound)
		return
	}
	if inst.Status != models.ServiceStatusAvailable {
		http.Error(w, fmt.Sprintf("service %q is not available (status: %s)", name, inst.Status), http.StatusBadRequest)
		return
	}

	if err := mgr.AddBinding(ctx, name, appName); err != nil {
		http.Error(w, "adding binding: "+err.Error(), http.StatusInternalServerError)
		return
	}

	secretName := "mf-svc-" + name
	if err := binder.Bind(ctx, appName, secretName); err != nil {
		_ = mgr.RemoveBinding(ctx, name, appName)
		http.Error(w, "binding to deployment: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/services/"+name)
	w.WriteHeader(http.StatusOK)
}

// UnbindServiceHandler unbinds a service instance from an app.
func (s *Server) UnbindServiceHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := r.PathValue("name")
	appName := r.FormValue("app_name")

	if appName == "" {
		http.Error(w, "app_name is required", http.StatusBadRequest)
		return
	}

	client, err := s.getClient(r)
	if err != nil {
		http.Error(w, "No cluster available: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	mgr := service.NewManager(client)
	binder := service.NewBinder(client)

	secretName := "mf-svc-" + name
	if err := binder.Unbind(ctx, appName, secretName); err != nil {
		http.Error(w, "unbinding from deployment: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := mgr.RemoveBinding(ctx, name, appName); err != nil {
		http.Error(w, "removing binding: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/services/"+name)
	w.WriteHeader(http.StatusOK)
}

// DeleteServiceHandler deletes a service instance.
func (s *Server) DeleteServiceHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := r.PathValue("name")

	client, err := s.getClient(r)
	if err != nil {
		http.Error(w, "No cluster available: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	mgr := service.NewManager(client)

	inst, err := mgr.Get(ctx, name)
	if err != nil {
		http.Error(w, "Service not found: "+err.Error(), http.StatusNotFound)
		return
	}

	if len(inst.Bindings) > 0 {
		http.Error(w, fmt.Sprintf("service %q has %d active binding(s) — unbind all apps first", name, len(inst.Bindings)), http.StatusBadRequest)
		return
	}

	if err := mgr.Delete(ctx, name); err != nil {
		http.Error(w, "deleting service: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// API handlers

// APIServicesListHandler returns service instances as JSON.
func (s *Server) APIServicesListHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	client, err := s.getClient(r)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}

	mgr := service.NewManager(client)
	items, err := mgr.List(ctx)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, items)
}

// APIServiceDetailHandler returns a single service instance as JSON.
func (s *Server) APIServiceDetailHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := r.PathValue("name")

	client, err := s.getClient(r)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}

	mgr := service.NewManager(client)
	inst, err := mgr.Get(ctx, name)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, inst)
}

// APIMarketplaceHandler returns the service catalog as JSON.
func (s *Server) APIMarketplaceHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, service.Catalog())
}
