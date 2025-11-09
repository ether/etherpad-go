package settings

type PublicSettings struct {
	GitVersion          string  `json:"gitVersion"`
	Toolbar             Toolbar `json:"toolbar"`
	ExposeVersion       bool    `json:"exposeVersion"`
	RandomVersionString string  `json:"randomVersionString"`
	Title               string  `json:"title"`
	SkinName            string  `json:"skinName"`
	SkinVariants        string  `json:"skinVariants"`
}
