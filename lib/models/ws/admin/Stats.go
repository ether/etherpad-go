package admin

type Stats struct {
	// TotalUsers is the connected users of every instance of the cluster.
	TotalUsers int `json:"totalUsers"`
	// Instances counts the instances; 1 without a cluster.
	Instances int `json:"instances"`
}
