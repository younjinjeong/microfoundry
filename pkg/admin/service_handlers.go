package admin

import (
	"fmt"
	"log"
	"net/http"

	"github.com/younjinjeong/microfoundry/pkg/models"
	"github.com/younjinjeong/microfoundry/pkg/service"
	"github.com/younjinjeong/microfoundry/pkg/terraform"
)

// CatalogPageItem represents one service type with enriched plan data for the admin catalog.
type CatalogPageItem struct {
	ServiceType  models.ServiceType
	Plans        []CatalogPlanItem
	VisibleCount int
}

// CatalogPlanItem represents a single plan with topology and visibility info.
type CatalogPlanItem struct {
	Plan        models.ServicePlan
	HasTopology bool
	TopologyVer int
	FileCount   int
	IsVisible   bool
	Provisioner string
}

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
		log.Printf("error listing services: %v", err)
		items = []models.ServiceListItem{}
	}

	// Load visible catalog for the creation form
	visStore := service.NewPlanVisibilityStore(client)
	catalog, err := visStore.VisibleCatalog(ctx)
	if err != nil {
		log.Printf("error loading visible catalog: %v", err)
		catalog = []models.ServiceType{}
	}

	data := s.pageData("Backing Services", "services")
	data.Content = map[string]any{
		"Services": items,
		"Catalog":  catalog,
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

// CatalogHandler shows the admin service catalog with topology and visibility info.
func (s *Server) CatalogHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	catalog := service.Catalog()

	client, err := s.getClient(r)
	if err != nil {
		http.Error(w, "No cluster available: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	// Load topology status for all plans
	topoStore := terraform.NewTopologyStore(client)
	topoItems, _ := topoStore.List(ctx, catalog)
	topoMap := make(map[string]models.TopologyListItem)
	for _, item := range topoItems {
		topoMap[models.PlanKey(item.ServiceType, item.PlanID)] = item
	}

	// Load visibility settings
	visStore := service.NewPlanVisibilityStore(client)
	visibility, _ := visStore.GetAll(ctx)

	// Build enriched catalog items
	var items []CatalogPageItem
	for _, svc := range catalog {
		pageItem := CatalogPageItem{ServiceType: svc}
		for _, plan := range svc.Plans {
			key := models.PlanKey(svc.ID, plan.ID)
			topo := topoMap[key]

			provisioner := "K8s Native"
			if topo.HasTopology {
				provisioner = fmt.Sprintf("Terraform v%d", topo.Version)
			}

			visible := visibility.Plans[key]
			if visible {
				pageItem.VisibleCount++
			}

			pageItem.Plans = append(pageItem.Plans, CatalogPlanItem{
				Plan:        plan,
				HasTopology: topo.HasTopology,
				TopologyVer: topo.Version,
				FileCount:   topo.FileCount,
				IsVisible:   visible,
				Provisioner: provisioner,
			})
		}
		items = append(items, pageItem)
	}

	data := s.pageData("Service Catalog", "catalog")
	data.Content = map[string]any{
		"Items":              items,
		"TerraformAvailable": terraform.IsAvailable(),
	}
	s.templates.Render(w, "catalog.html", data)
}

// CreateServiceHandler provisions a new service instance from the catalog.
func (s *Server) CreateServiceHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	serviceType := r.FormValue("service_type")
	plan := r.FormValue("plan")
	name := r.FormValue("name")

	if serviceType == "" || plan == "" || name == "" {
		http.Error(w, "service_type, plan, and name are required", http.StatusBadRequest)
		return
	}

	if !models.ValidServiceName.MatchString(name) {
		http.Error(w, "invalid service name: must be lowercase alphanumeric with hyphens, 2-42 characters", http.StatusBadRequest)
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

	s.metrics.ServiceProvision.WithLabelValues(serviceType, plan, "success").Inc()

	// Provisioning happens async in manager.Create — redirect to detail page
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
	binder := service.NewBinder(client, mgr)

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

	if err := binder.Bind(ctx, appName, name); err != nil {
		_ = mgr.RemoveBinding(ctx, name, appName)
		http.Error(w, "binding to deployment: "+err.Error(), http.StatusInternalServerError)
		return
	}

	s.metrics.ServiceBind.WithLabelValues(name, appName, "bind").Inc()

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
	binder := service.NewBinder(client, mgr)

	if err := binder.Unbind(ctx, appName, name); err != nil {
		http.Error(w, "unbinding from deployment: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := mgr.RemoveBinding(ctx, name, appName); err != nil {
		http.Error(w, "removing binding: "+err.Error(), http.StatusInternalServerError)
		return
	}

	s.metrics.ServiceBind.WithLabelValues(name, appName, "unbind").Inc()

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

// APIServiceDetailHandler returns a single service instance as JSON (credentials redacted).
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

	// Redact sensitive fields in API response
	redacted := inst.Redacted()
	writeJSON(w, http.StatusOK, redacted)
}

// APICatalogHandler returns the full service catalog as JSON.
func (s *Server) APICatalogHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, service.Catalog())
}

// APIVisibleCatalogHandler returns only user-visible plans as JSON.
func (s *Server) APIVisibleCatalogHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	client, err := s.getClient(r)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}

	visStore := service.NewPlanVisibilityStore(client)
	visible, err := visStore.VisibleCatalog(ctx)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, visible)
}

// TogglePlanVisibilityHandler toggles a single plan's user visibility (HTMX).
func (s *Server) TogglePlanVisibilityHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	serviceType := r.PathValue("type")
	planID := r.PathValue("plan")

	client, err := s.getClient(r)
	if err != nil {
		http.Error(w, "No cluster available: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	visible := r.FormValue("visible") == "true"

	visStore := service.NewPlanVisibilityStore(client)
	if err := visStore.SetPlanVisibility(ctx, serviceType, planID, visible); err != nil {
		http.Error(w, "Failed to update visibility: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return updated toggle HTML for HTMX swap
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if visible {
		fmt.Fprintf(w, `<button hx-post="/catalog/%s/%s/visibility" hx-swap="outerHTML" hx-vals='{"visible":"false"}'
			class="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-medium bg-green-100 text-green-800 hover:bg-green-200 cursor-pointer">
			<svg class="w-3 h-3 mr-1" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"/></svg>
			Visible</button>`, serviceType, planID)
	} else {
		fmt.Fprintf(w, `<button hx-post="/catalog/%s/%s/visibility" hx-swap="outerHTML" hx-vals='{"visible":"true"}'
			class="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-medium bg-gray-100 text-gray-500 hover:bg-gray-200 cursor-pointer">
			<svg class="w-3 h-3 mr-1" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M3.707 2.293a1 1 0 00-1.414 1.414l14 14a1 1 0 001.414-1.414l-14-14zM10 18a8 8 0 100-16 8 8 0 000 16z" clip-rule="evenodd"/></svg>
			Hidden</button>`, serviceType, planID)
	}
}

// ToggleServiceVisibilityHandler toggles all plans of a service type (HTMX).
func (s *Server) ToggleServiceVisibilityHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	serviceType := r.PathValue("type")

	client, err := s.getClient(r)
	if err != nil {
		http.Error(w, "No cluster available: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	visible := r.FormValue("visible") == "true"
	catalog := service.Catalog()

	visStore := service.NewPlanVisibilityStore(client)
	if err := visStore.SetServiceVisibility(ctx, serviceType, visible, catalog); err != nil {
		http.Error(w, "Failed to update visibility: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Full page reload to reflect all toggle changes
	w.Header().Set("HX-Redirect", "/catalog")
	w.WriteHeader(http.StatusOK)
}
