package keycloakclient

import (
	"context"

	"github.com/go-logr/logr"
	pkgErrors "github.com/pkg/errors"

	"github.com/epam/edp-keycloak-operator/pkg/client/keycloak"
)

type terminator struct {
	clientID, realmName string
	kClient             keycloak.Client
	log                 logr.Logger
}

func makeTerminator(clientID, realmName string, kClient keycloak.Client, log logr.Logger) *terminator {
	return &terminator{
		clientID:  clientID,
		realmName: realmName,
		kClient:   kClient,
		log:       log,
	}
}

func (t *terminator) GetLogger() logr.Logger {
	return t.log
}

func (t *terminator) DeleteResource(ctx context.Context) error {
	log := t.log.WithValues("keycloak client id", t.clientID)
	log.Info("Start deleting keycloak client...")

	if err := t.kClient.DeleteClient(ctx, t.clientID, t.realmName); err != nil {
		return pkgErrors.Wrap(err, "unable to delete kk client")
	}

	log.Info("client deletion done")

	return nil
}
