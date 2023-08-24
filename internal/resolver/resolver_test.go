package resolver_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/cert-manager/cert-manager/pkg/acme/webhook"
	"github.com/cert-manager/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	"github.com/stackitcloud/stackit-cert-manager-webhook/internal/repository"
	repository_mock "github.com/stackitcloud/stackit-cert-manager-webhook/internal/repository/mock"
	"github.com/stackitcloud/stackit-cert-manager-webhook/internal/resolver"
	resolver_mock "github.com/stackitcloud/stackit-cert-manager-webhook/internal/resolver/mock"
	stackitdnsclient "github.com/stackitcloud/stackit-dns-api-client-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/client-go/rest"
)

var (
	configJson       = &v1.JSON{Raw: []byte(`{"projectId":"test"}`)}
	challengeRequest = &v1alpha1.ChallengeRequest{
		Config: configJson,
	}
)

func TestName(t *testing.T) {
	t.Parallel()

	r := resolver.NewResolver(nil, nil, nil, nil, nil)

	assert.Equal(t, r.Name(), "stackit")
}

func TestInitialize(t *testing.T) {
	t.Parallel()

	r := resolver.NewResolver(nil, nil, nil, nil, nil)

	t.Run("successful init", func(t *testing.T) {
		t.Parallel()

		kubeConfig := &rest.Config{}
		err := r.Initialize(kubeConfig, nil)
		assert.NoError(t, err)
	})

	t.Run("unsuccessful init", func(t *testing.T) {
		t.Parallel()

		kubeConfig := &rest.Config{Burst: -1, RateLimiter: nil, QPS: 1}
		err := r.Initialize(kubeConfig, nil)
		assert.Error(t, err)
	})
}

type presentSuite struct {
	suite.Suite
	ctrl                       *gomock.Controller
	mockSecretFetcher          *resolver_mock.MockSecretFetcher
	mockConfigProvider         *resolver_mock.MockConfigProvider
	mockZoneRepositoryFactory  *repository_mock.MockZoneRepositoryFactory
	mockRRSetRepositoryFactory *repository_mock.MockRRSetRepositoryFactory
	mockZoneRepository         *repository_mock.MockZoneRepository
	mockRRSetRepository        *repository_mock.MockRRSetRepository
	resolver                   webhook.Solver
}

func (s *presentSuite) SetupTest() {
	s.mockSecretFetcher = resolver_mock.NewMockSecretFetcher(s.ctrl)
	s.mockConfigProvider = resolver_mock.NewMockConfigProvider(s.ctrl)
	s.mockZoneRepositoryFactory = repository_mock.NewMockZoneRepositoryFactory(s.ctrl)
	s.mockRRSetRepositoryFactory = repository_mock.NewMockRRSetRepositoryFactory(s.ctrl)
	s.mockZoneRepository = repository_mock.NewMockZoneRepository(s.ctrl)
	s.mockRRSetRepository = repository_mock.NewMockRRSetRepository(s.ctrl)

	s.resolver = resolver.NewResolver(
		&http.Client{},
		s.mockZoneRepositoryFactory,
		s.mockRRSetRepositoryFactory,
		s.mockSecretFetcher,
		s.mockConfigProvider,
	)
}

func (s *presentSuite) TearDownSuite() {
	s.ctrl.Finish()
}

func TestPresentTestSuite(t *testing.T) {
	t.Parallel()

	pSuite := new(presentSuite)
	pSuite.ctrl = gomock.NewController(t)

	suite.Run(t, pSuite)
}

func (s *presentSuite) TestConfigProviderError() {
	s.mockConfigProvider.EXPECT().
		LoadConfig(configJson).
		Return(resolver.StackitDnsProviderConfig{}, fmt.Errorf("error decoding solver configProvider"))

	err := s.resolver.Present(challengeRequest)
	s.Error(err)
}

func (s *presentSuite) TestFailGetAuthToken() {
	s.mockConfigProvider.EXPECT().
		LoadConfig(gomock.Any()).
		Return(resolver.StackitDnsProviderConfig{}, nil)
	s.mockSecretFetcher.EXPECT().
		StringFromSecret(gomock.Any(), gomock.Any(), gomock.Any()).
		Return("", fmt.Errorf("error fetching token"))

	err := s.resolver.Present(challengeRequest)
	s.Error(err)
	s.Containsf(
		err.Error(),
		"error fetching token",
		"error message should contain error from secretFetcher",
	)
}

func (s *presentSuite) TestFailFetchZone() {
	s.mockConfigProvider.EXPECT().
		LoadConfig(gomock.Any()).
		Return(resolver.StackitDnsProviderConfig{}, nil)
	s.mockSecretFetcher.EXPECT().
		StringFromSecret(gomock.Any(), gomock.Any(), gomock.Any()).
		Return("", nil)
	s.mockZoneRepositoryFactory.EXPECT().
		NewZoneRepository(gomock.Any()).
		Return(s.mockZoneRepository)
	s.mockZoneRepository.EXPECT().
		FetchZone(gomock.Any(), gomock.Any()).
		Return(nil, fmt.Errorf("error fetching zone"))

	err := s.resolver.Present(challengeRequest)
	s.Error(err)
	s.Containsf(
		err.Error(),
		"error fetching zone",
		"error message should contain error from zoneRepository",
	)
}

func (s *presentSuite) TestFailFetchRRSet() {
	s.mockConfigProvider.EXPECT().
		LoadConfig(gomock.Any()).
		Return(resolver.StackitDnsProviderConfig{}, nil)
	s.mockSecretFetcher.EXPECT().
		StringFromSecret(gomock.Any(), gomock.Any(), gomock.Any()).
		Return("", nil)
	s.mockZoneRepositoryFactory.EXPECT().
		NewZoneRepository(gomock.Any()).
		Return(s.mockZoneRepository)
	s.mockZoneRepository.EXPECT().
		FetchZone(gomock.Any(), gomock.Any()).
		Return(&stackitdnsclient.DomainZone{Id: "test"}, nil)
	s.mockRRSetRepositoryFactory.EXPECT().
		NewRRSetRepository(gomock.Any(), gomock.Any()).
		Return(s.mockRRSetRepository)
	s.mockRRSetRepository.EXPECT().
		FetchRRSetForZone(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, fmt.Errorf("error fetching rr set"))

	err := s.resolver.Present(challengeRequest)
	s.Error(err)
	s.Containsf(
		err.Error(),
		"error fetching rr set",
		"error message should contain error from rrSetRepository",
	)
}

func (s *presentSuite) TestSuccessCreateRRSet() {
	s.mockConfigProvider.EXPECT().
		LoadConfig(gomock.Any()).
		Return(resolver.StackitDnsProviderConfig{}, nil)
	s.mockSecretFetcher.EXPECT().
		StringFromSecret(gomock.Any(), gomock.Any(), gomock.Any()).
		Return("", nil)
	s.mockZoneRepositoryFactory.EXPECT().
		NewZoneRepository(gomock.Any()).
		Return(s.mockZoneRepository)
	s.mockZoneRepository.EXPECT().
		FetchZone(gomock.Any(), gomock.Any()).
		Return(&stackitdnsclient.DomainZone{Id: "test"}, nil)
	s.mockRRSetRepositoryFactory.EXPECT().
		NewRRSetRepository(gomock.Any(), gomock.Any()).
		Return(s.mockRRSetRepository)
	s.mockRRSetRepository.EXPECT().
		FetchRRSetForZone(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, repository.ErrRRSetNotFound)
	s.mockRRSetRepository.EXPECT().
		CreateRRSet(gomock.Any(), gomock.Any()).
		Return(nil)

	err := s.resolver.Present(challengeRequest)
	s.NoError(err)
}

func (s *presentSuite) TestSuccessUpdateRRSet() {
	s.mockConfigProvider.EXPECT().
		LoadConfig(gomock.Any()).
		Return(resolver.StackitDnsProviderConfig{}, nil)
	s.mockSecretFetcher.EXPECT().
		StringFromSecret(gomock.Any(), gomock.Any(), gomock.Any()).
		Return("", nil)
	s.mockZoneRepositoryFactory.EXPECT().
		NewZoneRepository(gomock.Any()).
		Return(s.mockZoneRepository)
	s.mockZoneRepository.EXPECT().
		FetchZone(gomock.Any(), gomock.Any()).
		Return(&stackitdnsclient.DomainZone{Id: "test"}, nil)
	s.mockRRSetRepositoryFactory.EXPECT().
		NewRRSetRepository(gomock.Any(), gomock.Any()).
		Return(s.mockRRSetRepository)
	s.mockRRSetRepository.EXPECT().
		FetchRRSetForZone(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&stackitdnsclient.DomainRrSet{}, nil)
	s.mockRRSetRepository.EXPECT().
		UpdateRRSet(gomock.Any(), gomock.Any()).
		Return(nil)

	err := s.resolver.Present(challengeRequest)
	s.NoError(err)
}