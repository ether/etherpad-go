package settings

import "testing"

func TestClusterAutoModeOnlyUnderAnOrchestrator(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	t.Setenv("POD_IP", "")
	auto := Cluster{Mode: "auto"}
	if auto.Enabled("postgres") {
		t.Fatal("an upgraded single instance on postgres must not start the cluster port")
	}
	if !(Cluster{Mode: "on"}).Enabled("postgres") {
		t.Fatal("mode on enables the cluster")
	}

	t.Setenv("KUBERNETES_SERVICE_HOST", "10.0.0.1")
	for db, want := range map[string]bool{"postgres": true, "mysql": true, "sqlite": false, "memory": false} {
		if got := auto.Enabled(db); got != want {
			t.Errorf("auto in Kubernetes with %s: %v, want %v", db, got, want)
		}
	}
	if (Cluster{Mode: "off"}).Enabled("postgres") {
		t.Fatal("mode off disables the cluster even in Kubernetes")
	}

	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	t.Setenv("POD_IP", "10.1.2.3")
	if !auto.Enabled("mysql") {
		t.Fatal("POD_IP marks an orchestrated instance")
	}
}
