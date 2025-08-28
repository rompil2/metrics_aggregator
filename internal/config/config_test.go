package config

import (
	"testing"
)

func TestNetAddress_Set(t *testing.T) {

	tests := []struct {
		name    string
		a       *NetAddress
		flagVal string
		wantErr bool
	}{
		{"Positive testcase", new(NetAddress), "localhost:8080", false},
		{"Positive testcase witout host", new(NetAddress), ":8080", false},
		{"Negativee testcase witout port", new(NetAddress), "localhost:", true},
		{"Negativee testcase port is too big number", new(NetAddress), "localhost:65536", true},
		{"Negativee testcase port is not a number", new(NetAddress), "localhost:6553i", true},
		{"Negativee testcase too many arguments", new(NetAddress), ":65536:xxx", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.a.Set(tt.flagVal); (err != nil) != tt.wantErr {
				t.Errorf("NetAddress.Set() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNetAddress_String(t *testing.T) {
	tests := []struct {
		name string
		Host string
		Port uint16
		want string
	}{
		// TODO: Add test cases.
		{"Positive test", "localhost", 8081, "localhost:8081"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NetAddress{tt.Host, tt.Port}
			if got := a.String(); got != tt.want {
				t.Errorf("NetAddress.String() = %v, want %v", got, tt.want)
			}
		})
	}
}
