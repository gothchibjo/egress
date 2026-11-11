package platform

import "testing"

func TestReadInterfaces(t *testing.T) {
	interfaces, err := ReadInterfaces()
	if err != nil {
		t.Fatalf("ReadInterfaces() error = %v", err)
	}
	if len(interfaces) == 0 {
		t.Fatal("ReadInterfaces() returned no interfaces")
	}
	loopback := false
	for _, iface := range interfaces {
		if iface.Name == "" {
			t.Fatalf("interface has no name: %+v", iface)
		}
		if iface.Loopback {
			loopback = true
		}
	}
	if !loopback {
		t.Fatal("ReadInterfaces() did not report the loopback interface")
	}
}
