package main

import (
	"testing"

	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

func TestAccessTokenVersion(t *testing.T) {
	api := models.NewApiApplication()
	if got := accessTokenVersion(nil); got != 1 {
		t.Errorf("missing API settings: got %d, want 1", got)
	}
	if got := accessTokenVersion(api); got != 1 {
		t.Errorf("unset version: got %d, want 1", got)
	}
	for _, version := range []int32{1, 2} {
		api.SetRequestedAccessTokenVersion(&version)
		if got := accessTokenVersion(api); got != version {
			t.Errorf("explicit version: got %d, want %d", got, version)
		}
	}
}
