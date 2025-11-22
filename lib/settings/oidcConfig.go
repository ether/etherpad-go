package settings

type OidcConfig struct {
	Authority   string   `json:"authority"`
	ClientId    string   `json:"clientId"`
	JwksUri     string   `json:"jwksUri"`
	RedirectUri string   `json:"redirectUri"`
	Scope       []string `json:"scope"`
}
