//go:build wireinject
// +build wireinject

package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/ent"
	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/bozhouDev/DragonCode-sub2api/internal/handler"
	"github.com/bozhouDev/DragonCode-sub2api/internal/repository"
	"github.com/bozhouDev/DragonCode-sub2api/internal/server"
	"github.com/bozhouDev/DragonCode-sub2api/internal/server/middleware"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"

	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
)

type Application struct {
	Server    *http.Server
	Readiness *server.Readiness
	Lifecycle *service.Lifecycle
	Cleanup   func()
}

func initializeApplication(buildInfo handler.BuildInfo) (*Application, error) {
	wire.Build(
		// Infrastructure layer ProviderSets
		config.ProviderSet,

		// Business layer ProviderSets
		repository.ProviderSet,
		service.ProviderSet,
		middleware.ProviderSet,
		handler.ProviderSet,

		// Server layer ProviderSet
		server.ProviderSet,

		// Privacy client factory for OpenAI training opt-out
		providePrivacyClientFactory,
		provideOpsGroupRepositories,

		// BuildInfo provider
		provideServiceBuildInfo,

		// Cleanup function provider
		provideCleanup,

		// Application struct
		wire.Struct(new(Application), "Server", "Readiness", "Lifecycle", "Cleanup"),
	)
	return nil, nil
}

func providePrivacyClientFactory() service.PrivacyClientFactory {
	return repository.CreatePrivacyReqClient
}

func provideServiceBuildInfo(buildInfo handler.BuildInfo) service.BuildInfo {
	return service.BuildInfo{
		Version:   buildInfo.Version,
		BuildType: buildInfo.BuildType,
	}
}

// Wire models a variadic constructor parameter as a slice dependency. Keep the
// generated graph reproducible while preserving NewOpsService's optional
// group-repository compatibility signature.
func provideOpsGroupRepositories(groupRepo service.GroupRepository) []service.GroupRepository {
	return []service.GroupRepository{groupRepo}
}

func provideCleanup(
	entClient *ent.Client,
	rdb *redis.Client,
	lifecycle *service.Lifecycle,
) func() {
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if lifecycle != nil {
			if err := lifecycle.Stop(ctx); err != nil {
				log.Printf("[Cleanup] lifecycle stop failed: %v", err)
			}
		}
		// Infrastructure is always closed after every registered dependent.
		if rdb != nil {
			if err := rdb.Close(); err != nil {
				log.Printf("[Cleanup] Redis close failed: %v", err)
			}
		}
		if entClient != nil {
			if err := entClient.Close(); err != nil {
				log.Printf("[Cleanup] Ent close failed: %v", err)
			}
		}
	}
}
