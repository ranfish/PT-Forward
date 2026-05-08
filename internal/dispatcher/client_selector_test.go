package dispatcher

import (
	"context"
	"testing"

	"github.com/ranfish/pt-forward/internal/mocks"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

func TestClientSelector_Fixed(t *testing.T) {
	s := NewClientSelector(nil, zap.NewNop())
	sub := &model.RSSSubscription{
		ClientID:        "client-a",
		ClientSelection: model.SelectionFixed,
	}
	selected, err := s.Select(context.Background(), sub)
	if err != nil {
		t.Fatal(err)
	}
	if selected != "client-a" {
		t.Errorf("fixed mode should return client-a, got %s", selected)
	}
}

func TestClientSelector_EmptyCandidates(t *testing.T) {
	s := NewClientSelector(nil, zap.NewNop())
	sub := &model.RSSSubscription{
		ClientID:         "fallback",
		CandidateClients: []string{},
		ClientSelection:  model.SelectionMostSpace,
	}
	selected, err := s.Select(context.Background(), sub)
	if err != nil {
		t.Fatal(err)
	}
	if selected != "fallback" {
		t.Errorf("empty candidates should fallback, got %s", selected)
	}
}

func TestClientSelector_MostSpace(t *testing.T) {
	provider := &mocks.DownloaderProvider{
		GetFn: func(id string) (model.DownloaderClient, error) {
			clients := map[string]*mocks.DownloaderClient{
				"a": {Name: "a", GetMainDataFn: func(ctx context.Context) (*model.Maindata, error) {
					return &model.Maindata{FreeSpace: 1 * 1024 * 1024 * 1024}, nil
				}},
				"b": {Name: "b", GetMainDataFn: func(ctx context.Context) (*model.Maindata, error) {
					return &model.Maindata{FreeSpace: 10 * 1024 * 1024 * 1024}, nil
				}},
			}
			c, ok := clients[id]
			if !ok {
				return nil, &model.AppError{Code: 40400, Message: "not found"}
			}
			return c, nil
		},
	}
	s := NewClientSelector(provider, zap.NewNop())
	sub := &model.RSSSubscription{
		ClientID:         "a",
		CandidateClients: []string{"a", "b"},
		ClientSelection:  model.SelectionMostSpace,
	}

	selected, err := s.Select(context.Background(), sub)
	if err != nil {
		t.Fatal(err)
	}
	if selected != "b" {
		t.Errorf("most_space should select client with most free space, got %s", selected)
	}
}

func TestClientSelector_LeastLoad(t *testing.T) {
	provider := &mocks.DownloaderProvider{
		GetFn: func(id string) (model.DownloaderClient, error) {
			clients := map[string]*mocks.DownloaderClient{
				"a": {Name: "a", GetMainDataFn: func(ctx context.Context) (*model.Maindata, error) {
					return &model.Maindata{ServerState: model.ServerState{UploadSpeed: 15 * 1024 * 1024}}, nil
				}},
				"b": {Name: "b", GetMainDataFn: func(ctx context.Context) (*model.Maindata, error) {
					return &model.Maindata{ServerState: model.ServerState{UploadSpeed: 1 * 1024 * 1024}}, nil
				}},
			}
			c, ok := clients[id]
			if !ok {
				return nil, &model.AppError{Code: 40400, Message: "not found"}
			}
			return c, nil
		},
	}
	s := NewClientSelector(provider, zap.NewNop())
	sub := &model.RSSSubscription{
		ClientID:         "a",
		CandidateClients: []string{"a", "b"},
		ClientSelection:  model.SelectionLeastLoad,
	}

	selected, err := s.Select(context.Background(), sub)
	if err != nil {
		t.Fatal(err)
	}
	if selected != "b" {
		t.Errorf("least_load should select client with lowest upload speed, got %s", selected)
	}
}

func TestClientSelector_RoundRobin(t *testing.T) {
	provider := &mocks.DownloaderProvider{
		GetFn: func(id string) (model.DownloaderClient, error) {
			clients := map[string]*mocks.DownloaderClient{
				"a": {Name: "a"},
				"b": {Name: "b"},
				"c": {Name: "c"},
			}
			c, ok := clients[id]
			if !ok {
				return nil, &model.AppError{Code: 40400, Message: "not found"}
			}
			return c, nil
		},
	}
	s := NewClientSelector(provider, zap.NewNop())
	sub := &model.RSSSubscription{
		Name:             "test-sub",
		ClientID:         "a",
		CandidateClients: []string{"a", "b", "c"},
		ClientSelection:  model.SelectionRoundRobin,
	}

	results := make(map[string]int)
	for i := 0; i < 6; i++ {
		selected, _ := s.Select(context.Background(), sub)
		results[selected]++
	}

	if len(results) < 2 {
		t.Errorf("round_robin should distribute across clients, got %v", results)
	}
}

func TestClientSelector_BestFit(t *testing.T) {
	provider := &mocks.DownloaderProvider{
		GetFn: func(id string) (model.DownloaderClient, error) {
			clients := map[string]*mocks.DownloaderClient{
				"a": {Name: "a", GetMainDataFn: func(ctx context.Context) (*model.Maindata, error) {
					return &model.Maindata{
						FreeSpace:   100 * 1024 * 1024 * 1024,
						ServerState: model.ServerState{UploadSpeed: 50 * 1024 * 1024},
					}, nil
				}},
				"b": {Name: "b", GetMainDataFn: func(ctx context.Context) (*model.Maindata, error) {
					return &model.Maindata{
						FreeSpace:   500 * 1024 * 1024 * 1024,
						ServerState: model.ServerState{UploadSpeed: 5 * 1024 * 1024},
					}, nil
				}},
			}
			c, ok := clients[id]
			if !ok {
				return nil, &model.AppError{Code: 40400, Message: "not found"}
			}
			return c, nil
		},
	}
	s := NewClientSelector(provider, zap.NewNop())
	sub := &model.RSSSubscription{
		ClientID:         "a",
		CandidateClients: []string{"a", "b"},
		ClientSelection:  model.SelectionBestFit,
	}

	selected, err := s.Select(context.Background(), sub)
	if err != nil {
		t.Fatal(err)
	}
	if selected != "b" {
		t.Errorf("best_fit should select client with lowest upload + most space, got %s", selected)
	}
}

func TestClientSelector_BestFit_AllFull(t *testing.T) {
	provider := &mocks.DownloaderProvider{
		GetFn: func(id string) (model.DownloaderClient, error) {
			clients := map[string]*mocks.DownloaderClient{
				"a": {Name: "a", GetMainDataFn: func(ctx context.Context) (*model.Maindata, error) {
					return &model.Maindata{
						FreeSpace:   1 * 1024 * 1024 * 1024,
						ServerState: model.ServerState{UploadSpeed: 0},
					}, nil
				}},
				"b": {Name: "b", GetMainDataFn: func(ctx context.Context) (*model.Maindata, error) {
					return &model.Maindata{
						FreeSpace:   2 * 1024 * 1024 * 1024,
						ServerState: model.ServerState{UploadSpeed: 0},
					}, nil
				}},
			}
			c, ok := clients[id]
			if !ok {
				return nil, &model.AppError{Code: 40400, Message: "not found"}
			}
			return c, nil
		},
	}
	s := NewClientSelector(provider, zap.NewNop())
	sub := &model.RSSSubscription{
		ClientID:         "a",
		CandidateClients: []string{"a", "b"},
		ClientSelection:  model.SelectionBestFit,
	}

	selected, err := s.Select(context.Background(), sub)
	if err != nil {
		t.Fatal(err)
	}
	if selected != "b" {
		t.Errorf("best_fit with equal upload should prefer more space, got %s", selected)
	}
}

func TestClientSelector_LeastUpload_EmptyTorrents(t *testing.T) {
	provider := &mocks.DownloaderProvider{
		GetFn: func(id string) (model.DownloaderClient, error) {
			clients := map[string]*mocks.DownloaderClient{
				"a": {Name: "a", GetMainDataFn: func(ctx context.Context) (*model.Maindata, error) {
					return &model.Maindata{ServerState: model.ServerState{UploadSpeed: 0}}, nil
				}},
				"b": {Name: "b", GetMainDataFn: func(ctx context.Context) (*model.Maindata, error) {
					return &model.Maindata{ServerState: model.ServerState{UploadSpeed: 1024}}, nil
				}},
			}
			c, ok := clients[id]
			if !ok {
				return nil, &model.AppError{Code: 40400, Message: "not found"}
			}
			return c, nil
		},
	}
	s := NewClientSelector(provider, zap.NewNop())
	sub := &model.RSSSubscription{
		ClientID:         "a",
		CandidateClients: []string{"a", "b"},
		ClientSelection:  model.SelectionLeastLoad,
	}

	selected, err := s.Select(context.Background(), sub)
	if err != nil {
		t.Fatal(err)
	}
	if selected != "a" {
		t.Errorf("least_load should prefer client with zero upload, got %s", selected)
	}
}
