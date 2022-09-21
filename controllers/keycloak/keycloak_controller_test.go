package keycloak

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Nerzal/gocloak/v10"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	edpCompApi "github.com/epam/edp-component-operator/pkg/apis/v1/v1"

	keycloakApi "github.com/epam/edp-keycloak-operator/api/v1/v1"
	"github.com/epam/edp-keycloak-operator/controllers/helper"
	"github.com/epam/edp-keycloak-operator/pkg/client/keycloak/adapter"
	"github.com/epam/edp-keycloak-operator/pkg/client/keycloak/mock"
)

func TestNewReconcileKeycloak(t *testing.T) {
	kc := NewReconcileKeycloak(nil, nil, mock.NewLogr(), &helper.Mock{})
	if kc.scheme != nil {
		t.Fatal("something went wrong")
	}
}

func TestReconcileKeycloak_ReconcileInvalidSpec(t *testing.T) {
	cr := &keycloakApi.Keycloak{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Keycloak",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "NewKeycloak",
			Namespace: "namespace",
		},
		Spec: keycloakApi.KeycloakSpec{
			Url:    "https://some",
			Secret: "keycloak-secret",
		},
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "keycloak-secret",
			Namespace: "namespace",
		},
		Data: map[string][]byte{
			"username": []byte("user"),
			"password": []byte("pass"),
		},
	}
	objs := []runtime.Object{
		cr, secret,
	}
	s := scheme.Scheme
	s.AddKnownTypes(v1.SchemeGroupVersion, cr, &keycloakApi.KeycloakRealm{}, &edpCompApi.EDPComponent{})

	cl := fake.NewClientBuilder().WithRuntimeObjects(objs...).Build()

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "NewKeycloak",
			Namespace: "namespace",
		},
	}

	logger := mock.NewLogr()
	h := helper.Mock{}
	h.On("CreateKeycloakClientFromTokenSecret", cr).
		Return(nil, adapter.TokenExpiredError("token expired"))
	h.On("CreateKeycloakClientFromLoginPassword", cr).Return(nil, errors.New("fatal"))

	r := ReconcileKeycloak{
		client: cl,
		scheme: s,
		log:    logger,
		helper: &h,
	}

	res, err := r.Reconcile(context.TODO(), req)

	assert.NoError(t, err)
	assert.False(t, res.Requeue)

	persisted := &keycloakApi.Keycloak{}
	err = cl.Get(context.TODO(), req.NamespacedName, persisted)
	assert.Nil(t, err)
	assert.False(t, persisted.Status.Connected)

	realm := &keycloakApi.KeycloakRealm{}
	nsnRealm := types.NamespacedName{
		Name:      "main",
		Namespace: "namespace",
	}
	err = cl.Get(context.TODO(), nsnRealm, realm)

	assert.Error(t, err)

	assert.True(t, k8sErrors.IsNotFound(err))
}

func TestReconcileKeycloak_ReconcileCreateMainRealm(t *testing.T) {
	cr := &keycloakApi.Keycloak{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Keycloak",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{Name: "NewKeycloak", Namespace: "namespace"},
		Spec:       keycloakApi.KeycloakSpec{Url: "https://some", Secret: "keycloak-secret"}}
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "keycloak-secret", Namespace: "namespace"},
		Data: map[string][]byte{"username": []byte("user"), "password": []byte("pass")},
	}
	comp := &edpCompApi.EDPComponent{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%v-keycloak", cr.Name),
		Namespace: cr.Namespace}}
	s := scheme.Scheme
	s.AddKnownTypes(v1.SchemeGroupVersion, cr, &keycloakApi.KeycloakRealm{}, comp)
	cl := fake.NewClientBuilder().WithRuntimeObjects(cr, secret, comp).Build()

	req := reconcile.Request{NamespacedName: types.NamespacedName{Name: cr.Name, Namespace: cr.Namespace}}

	kClient := adapter.Mock{}
	logger := mock.NewLogr()
	h := helper.Mock{}
	h.On("CreateKeycloakClientFromTokenSecret", cr).
		Return(nil, adapter.TokenExpiredError("token expired"))
	h.On("CreateKeycloakClientFromLoginPassword", cr).Return(&kClient, nil)

	r := ReconcileKeycloak{
		client: cl,
		scheme: s,
		log:    logger,
		helper: &h,
	}

	_, err := r.Reconcile(context.TODO(), req)
	if err != nil {
		t.Fatalf("%+v", err)
	}

	err = cl.Get(context.Background(),
		types.NamespacedName{Namespace: cr.Namespace, Name: "main"}, &keycloakApi.KeycloakRealm{})
	require.NoError(t, err)
}

func TestReconcileKeycloak_ReconcileDontCreateMainRealm(t *testing.T) {
	cr := &keycloakApi.Keycloak{TypeMeta: metav1.TypeMeta{
		Kind:       "Keycloak",
		APIVersion: "apps/v1",
	}, ObjectMeta: metav1.ObjectMeta{Name: "NewKeycloak", Namespace: "namespace"},
		Spec: keycloakApi.KeycloakSpec{Url: "https://some", Secret: "keycloak-secret",
			InstallMainRealm: gocloak.BoolP(false)}}
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "keycloak-secret", Namespace: "namespace"},
		Data: map[string][]byte{"username": []byte("user"), "password": []byte("pass")},
	}
	comp := &edpCompApi.EDPComponent{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%v-keycloak", cr.Name),
		Namespace: cr.Namespace}}
	s := scheme.Scheme
	s.AddKnownTypes(v1.SchemeGroupVersion, cr, &keycloakApi.KeycloakRealm{}, comp)
	cl := fake.NewClientBuilder().WithRuntimeObjects(cr, secret, comp).Build()

	req := reconcile.Request{NamespacedName: types.NamespacedName{Name: cr.Name, Namespace: cr.Namespace}}
	kClient := adapter.Mock{}
	logger := mock.NewLogr()
	h := helper.Mock{}
	h.On("CreateKeycloakClientFromTokenSecret", cr).
		Return(nil, adapter.TokenExpiredError("token expired"))
	h.On("CreateKeycloakClientFromLoginPassword", cr).Return(&kClient, nil)

	r := ReconcileKeycloak{
		client:                  cl,
		scheme:                  s,
		log:                     logger,
		helper:                  &h,
		successReconcileTimeout: time.Hour,
	}

	res, err := r.Reconcile(context.TODO(), req)
	if err != nil {
		t.Fatalf("%+v", err)
	}

	if res.RequeueAfter != r.successReconcileTimeout {
		t.Fatal("result RequeueAfter is not set")
	}

	if err := cl.Get(context.Background(),
		types.NamespacedName{Namespace: cr.Namespace, Name: "main"}, &keycloakApi.KeycloakRealm{}); err == nil {
		t.Fatal("main realm has been created")
	}
}

func TestReconcileKeycloak_Reconcile_FailureGetInstance(t *testing.T) {
	cr := &keycloakApi.Keycloak{ObjectMeta: metav1.ObjectMeta{Name: "NewKeycloak", Namespace: "namespace"},
		Spec: keycloakApi.KeycloakSpec{Url: "https://some", Secret: "keycloak-secret",
			InstallMainRealm: gocloak.BoolP(false)}}

	s := scheme.Scheme
	s.AddKnownTypes(v1.SchemeGroupVersion, cr, &keycloakApi.KeycloakRealm{})
	cl := fake.NewClientBuilder().WithRuntimeObjects(cr).Build()

	logger := mock.NewLogr()
	r := ReconcileKeycloak{
		client: cl,
		scheme: s,
		log:    logger,
	}

	rq := reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo23", Namespace: "bar23"}}

	res, err := r.Reconcile(context.Background(), rq)
	require.NoError(t, err)

	if res.RequeueAfter != 0 {
		t.Fatal("RequeueAfter is set")
	}

	if _, ok := logger.GetSink().(*mock.Logger).InfoMessages()["instance not found"]; !ok {
		t.Fatal("not found message is not logged")
	}

	mockK8S := helper.K8SClientMock{}
	r.client = &mockK8S

	var kc keycloakApi.Keycloak

	mockK8S.On("Get", rq.NamespacedName, &kc).Return(errors.New("get keycloak fatal"))

	res, err = r.Reconcile(context.Background(), rq)
	require.NoError(t, err)

	if res.RequeueAfter == 0 {
		t.Fatal("RequeueAfter is not set")
	}

	loggerSink, ok := logger.GetSink().(*mock.Logger)
	require.True(t, ok, "wrong logger type")
	require.Error(t, loggerSink.LastError())
	assert.Contains(t, "get keycloak fatal", loggerSink.LastError().Error())
}

func TestReconcileKeycloak_Reconcile_FailureUpdateConnectionStatusToKeycloak(t *testing.T) {
	cr := &keycloakApi.Keycloak{TypeMeta: metav1.TypeMeta{
		Kind:       "Keycloak",
		APIVersion: "apps/v1",
	}, ObjectMeta: metav1.ObjectMeta{Name: "NewKeycloak", Namespace: "namespace"},
		Spec: keycloakApi.KeycloakSpec{Url: "https://some", Secret: "keycloak-secret",
			InstallMainRealm: gocloak.BoolP(false)}}

	s := scheme.Scheme
	s.AddKnownTypes(v1.SchemeGroupVersion, cr, &keycloakApi.KeycloakRealm{})
	cl := fake.NewClientBuilder().WithRuntimeObjects(cr).Build()

	logger := mock.NewLogr()
	h := helper.Mock{}
	h.On("CreateKeycloakClientFromTokenSecret", cr).
		Return(nil, adapter.TokenExpiredError("token expired"))
	h.On("CreateKeycloakClientFromLoginPassword", cr).Return(nil,
		errors.New(`secrets "keycloak-secret" not found`))

	r := ReconcileKeycloak{
		client: cl,
		scheme: s,
		log:    logger,
		helper: &h,
	}

	rq := reconcile.Request{NamespacedName: types.NamespacedName{Name: cr.Name, Namespace: cr.Namespace}}

	_, err := r.Reconcile(context.Background(), rq)
	require.NoError(t, err)

	loggerSink, ok := logger.GetSink().(*mock.Logger)
	require.True(t, ok, "wrong logger type")
	require.Error(t, loggerSink.LastError())
	assert.Contains(t, loggerSink.LastError().Error(), "Keycloak CR status is not connected")
}

func TestReconcileKeycloak_Reconcile_FailureIsStatusConnected(t *testing.T) {
	cl := helper.K8SClientMock{}
	hm := helper.Mock{}
	kClMock := adapter.Mock{}
	logger := mock.NewLogr()
	err := keycloakApi.AddToScheme(scheme.Scheme)
	require.NoError(t, err)

	kc := keycloakApi.Keycloak{ObjectMeta: metav1.ObjectMeta{Namespace: "kc-ns1", Name: "kc-name-1"},
		Spec: keycloakApi.KeycloakSpec{Secret: "kc-secret-name-1"}}
	rq := reconcile.Request{NamespacedName: types.NamespacedName{Name: kc.Name, Namespace: kc.Namespace}}

	fakeCl := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(&kc).Build()

	r := ReconcileKeycloak{
		client: &cl,
		scheme: scheme.Scheme,
		log:    logger,
		helper: &hm,
	}

	cl.On("Status").Return(fakeCl)
	cl.On("Get", rq.NamespacedName, &kc).Return(fakeCl).Once()
	cl.On("Get", types.NamespacedName{Namespace: kc.Namespace, Name: kc.Spec.Secret},
		&corev1.Secret{}).Return(nil)

	hm.On("CreateKeycloakClientFromTokenSecret", &kc).
		Return(nil, adapter.TokenExpiredError("token expired"))
	hm.On("CreateKeycloakClientFromLoginPassword", &kc).Return(&kClMock, nil)

	cl.On("Get", rq.NamespacedName, &keycloakApi.Keycloak{}).
		Return(errors.New("isStatusConnected fatal")).Once()

	_, err = r.Reconcile(context.Background(), rq)
	require.NoError(t, err)

	loggerSink, ok := logger.GetSink().(*mock.Logger)
	require.True(t, ok, "wrong logger type")
	require.Error(t, loggerSink.LastError())
	assert.Contains(t, loggerSink.LastError().Error(), "isStatusConnected fatal")
}

func TestReconcileKeycloak_Reconcile_FailurePutMainRealm(t *testing.T) {
	cl := helper.K8SClientMock{}
	sch := runtime.NewScheme()

	err := keycloakApi.AddToScheme(sch)
	require.NoError(t, err)

	hm := helper.Mock{}
	kClMock := adapter.Mock{}
	logger := mock.NewLogr()

	kc := keycloakApi.Keycloak{Status: keycloakApi.KeycloakStatus{Connected: true}, ObjectMeta: metav1.ObjectMeta{
		Name:      "kc-main",
		Namespace: "kc-ns",
	}, Spec: keycloakApi.KeycloakSpec{Secret: "kc-secret-name"},
		TypeMeta: metav1.TypeMeta{APIVersion: "v1.edp.epam.com/v1", Kind: "Keycloak"}}

	rq := reconcile.Request{NamespacedName: types.NamespacedName{Name: kc.Name, Namespace: kc.Namespace}}
	sec := corev1.Secret{}
	realmCr := keycloakApi.KeycloakRealm{}

	fakeCl := fake.NewClientBuilder().WithScheme(sch).WithRuntimeObjects(&kc).Build()
	cl.On("Scheme").Return(fakeCl)
	cl.On("Status").Return(fakeCl)

	cl.On("Get", rq.NamespacedName, &keycloakApi.Keycloak{}).Return(fakeCl).Once()
	cl.On("Get", types.NamespacedName{}, &sec).Return(nil)
	hm.On("CreateKeycloakClientFromTokenSecret", &kc).
		Return(nil, adapter.TokenExpiredError("token expired"))
	hm.On("CreateKeycloakClientFromLoginPassword", &kc).Return(&kClMock, nil)

	cl.On("Get", rq.NamespacedName, &keycloakApi.Keycloak{}).Return(fakeCl).Once()
	cl.On("Get", types.NamespacedName{Name: "main", Namespace: kc.Namespace}, &realmCr).
		Return(errors.New("get main realm fatal"))

	r := ReconcileKeycloak{
		client: &cl,
		scheme: cl.Scheme(),
		log:    logger,
		helper: &hm,
	}

	_, err = r.Reconcile(context.Background(), rq)
	require.NoError(t, err)

	loggerSink, ok := logger.GetSink().(*mock.Logger)
	require.True(t, ok, "wrong logger type")
	require.Error(t, loggerSink.LastError())
	assert.Contains(t, loggerSink.LastError().Error(), "get main realm fatal")
}