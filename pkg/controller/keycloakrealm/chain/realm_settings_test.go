package chain

import (
	"strings"
	"testing"

	"github.com/pkg/errors"

	keycloakApi "github.com/epam/edp-keycloak-operator/pkg/apis/v1/v1alpha1"
	"github.com/epam/edp-keycloak-operator/pkg/client/keycloak/adapter"
)

func TestRealmSettings_ServeRequest(t *testing.T) {
	rs := RealmSettings{}
	kClient := new(adapter.Mock)
	realm := keycloakApi.KeycloakRealm{}

	if err := rs.ServeRequest(&realm, kClient); err != nil {
		t.Fatal(err)
	}

	realm = keycloakApi.KeycloakRealm{
		Spec: keycloakApi.KeycloakRealmSpec{
			RealmName: "realm1",
			Themes: &keycloakApi.RealmThemes{
				LoginTheme: stringP("LoginTheme test"),
			},
			BrowserSecurityHeaders: &map[string]string{
				"foo": "bar",
			},
			RealmEventConfig: &keycloakApi.RealmEventConfig{
				EventsListeners: []string{"foo", "bar"},
			},
		},
	}

	kClient.On("UpdateRealmSettings", realm.Spec.RealmName, &adapter.RealmSettings{
		Themes: &adapter.RealmThemes{
			LoginTheme: stringP("LoginTheme test"),
		},
		BrowserSecurityHeaders: &map[string]string{
			"foo": "bar",
		},
	}).Return(nil)

	kClient.On("SetRealmEventConfig", realm.Spec.RealmName, &adapter.RealmEventConfig{
		EventsListeners: []string{"foo", "bar"},
	}).Return(nil).Once()

	if err := rs.ServeRequest(&realm, kClient); err != nil {
		t.Fatal(err)
	}

	kClient.On("SetRealmEventConfig", realm.Spec.RealmName, &adapter.RealmEventConfig{
		EventsListeners: []string{"foo", "bar"},
	}).Return(errors.New("event config fatal")).Once()

	err := rs.ServeRequest(&realm, kClient)
	if err == nil {
		t.Fatal("no error returned")
	}

	if !strings.Contains(err.Error(), "unable to set realm event config") {
		t.Fatalf("wrong error returned: %s", err.Error())
	}
}