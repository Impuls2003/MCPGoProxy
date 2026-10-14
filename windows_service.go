package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const serviceName = "MCPGoProxy"

type mcpService struct{}

// Проверяет запущена ли программа как windows служба
func runAsWindowsService() (bool, error) {
	isService, err := svc.IsWindowsService()
	if err != nil || !isService {
		return false, err
	}
	err = svc.Run(serviceName, mcpService{})
	return true, err
}

// Установка как службы windows
func installWindowsService() error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}

	manager, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer manager.Disconnect()

	service, err := manager.OpenService(serviceName)
	if err == nil {
		service.Close()
		return fmt.Errorf("service %s already exists", serviceName)
	}

	service, err = manager.CreateService(serviceName, executable, mgr.Config{
		DisplayName: "MCPGoProxy",
		Description: "MCP proxy for ERP",
		StartType:   mgr.StartAutomatic,
	}, "--http")
	if err != nil {
		return err
	}
	defer service.Close()

	return service.Start()
}

// Удаление windows службы
func removeWindowsService() error {
	manager, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer manager.Disconnect()

	service, err := manager.OpenService(serviceName)
	if err != nil {
		return err
	}
	defer service.Close()

	_, err = service.Control(svc.Stop)
	if err != nil {
		// Служба могла быть уже остановлена.
	}

	return service.Delete()
}

func (mcpService) Execute(
	args []string,
	requests <-chan svc.ChangeRequest,
	statuses chan<- svc.Status,
) (bool, uint32) {
	const accepted = svc.AcceptStop | svc.AcceptShutdown

	statuses <- svc.Status{State: svc.StartPending}

	stop := make(chan struct{})
	done := make(chan error, 1)

	go func() {
		done <- runHTTPService(stop)
	}()

	statuses <- svc.Status{
		State:   svc.Running,
		Accepts: accepted,
	}

	for {
		select {
		case err := <-done:
			if err != nil {
				fmt.Println("HTTP service error:", err)
				return true, 1
			}
			return false, 0

		case request, ok := <-requests:
			if !ok {
				close(stop)
				<-done
				return false, 0
			}

			switch request.Cmd {
			case svc.Interrogate:
				statuses <- request.CurrentStatus

			case svc.Stop, svc.Shutdown:
				statuses <- svc.Status{State: svc.StopPending}
				close(stop)

				select {
				case err := <-done:
					if err != nil {
						fmt.Println("HTTP service shutdown error:", err)
						return true, 1
					}
					return false, 0

				case <-time.After(12 * time.Second):
					fmt.Println("HTTP service shutdown timed out")
					return true, 1
				}
			}
		}
	}
}

func runHTTPService(stop <-chan struct{}) error {

	cfg := LoadConfig()
	s := newMCPServer(cfg)
	httpServer := newHTTPServer(s, cfg)

	addr := cfg.HTTPHost + ":" + strconv.Itoa(cfg.HTTPPort)
	fmt.Println("Streamable HTTP service:", addr+"/mcp")

	done := make(chan error, 1)
	go func() {
		done <- httpServer.Start(addr)
	}()

	select {
	case err := <-done:
		return err

	case <-stop:
		ctx, cancel := context.WithTimeout(
			context.Background(),
			10*time.Second,
		)
		defer cancel()

		httpServer.CloseSessions(ctx)

		if err := httpServer.Shutdown(ctx); err != nil {
			return err
		}

		err := <-done
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
