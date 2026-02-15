package service

import "github.com/younjinjeong/microfoundry/pkg/models"

// Catalog returns all available service types.
func Catalog() []models.ServiceType {
	return []models.ServiceType{
		{
			ID:          "aws-rds-aurora-mariadb",
			Name:        "AWS RDS Aurora MariaDB",
			Description: "Managed MySQL-compatible relational database powered by Amazon Aurora",
			Provider:    "aws",
			Category:    "database",
			Tags:        []string{"mysql", "mariadb", "aurora", "rds", "relational"},
			Plans: []models.ServicePlan{
				{
					ID:            "small",
					Name:          "Small (Dev/Test)",
					Description:   "Single instance, minimal resources for development and testing",
					InstanceClass: "db.t4g.micro",
					StorageGB:     20,
					Replicas:      0,
					MultiAZ:       false,
					CostEstimate:  "~$15/month",
					Customizable:  false,
				},
				{
					ID:            "medium",
					Name:          "Medium (UAT/Staging)",
					Description:   "Moderate resources for UAT and staging environments",
					InstanceClass: "db.r6g.large",
					StorageGB:     100,
					Replicas:      0,
					MultiAZ:       false,
					CostEstimate:  "~$200/month",
					Customizable:  false,
				},
				{
					ID:            "production",
					Name:          "Production",
					Description:   "Production-grade with read replicas, Multi-AZ, and customizable sizing",
					InstanceClass: "db.r6g.xlarge",
					StorageGB:     200,
					Replicas:      1,
					MultiAZ:       true,
					CostEstimate:  "~$500+/month",
					Customizable:  true,
					Defaults: map[string]string{
						"instance_class": "db.r6g.xlarge",
						"storage_gb":     "200",
						"replicas":       "1",
					},
				},
			},
		},
	}
}

// FindServiceType returns a service type by ID.
func FindServiceType(id string) (models.ServiceType, bool) {
	for _, st := range Catalog() {
		if st.ID == id {
			return st, true
		}
	}
	return models.ServiceType{}, false
}

// FindPlan returns a plan within a service type.
func FindPlan(serviceTypeID, planID string) (models.ServicePlan, bool) {
	st, ok := FindServiceType(serviceTypeID)
	if !ok {
		return models.ServicePlan{}, false
	}
	for _, p := range st.Plans {
		if p.ID == planID {
			return p, true
		}
	}
	return models.ServicePlan{}, false
}
