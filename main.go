package main

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

func main() {
	apiClient, err := client.NewClientWithOpts(client.WithVersion("1.41"), client.FromEnv)
	if err != nil {
		panic(err)
	}
	defer apiClient.Close()

	containers, err := apiClient.ContainerList(context.Background(), container.ListOptions{})
	if err != nil {
		panic(err)
	}

	for _, ctr := range containers {
		fmt.Printf("%s (status: %s)\n", ctr.Image, ctr.Status)
	}
}
