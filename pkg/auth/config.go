package auth

// AuthConfig holds OIDC/Keycloak authentication settings.
type AuthConfig struct {
	Enabled      bool   `mapstructure:"enabled"`
	IssuerURL    string `mapstructure:"issuer_url"`
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
	RedirectURL  string `mapstructure:"redirect_url"`
	SessionKey   string `mapstructure:"session_key"`

	// Keycloak Admin API (for user CRUD and SCIM)
	AdminBaseURL      string `mapstructure:"admin_base_url"`
	AdminClientID     string `mapstructure:"admin_client_id"`
	AdminClientSecret string `mapstructure:"admin_client_secret"`
	Realm             string `mapstructure:"realm"`
}
