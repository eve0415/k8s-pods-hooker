package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

type UpdateDeploymentRequest struct {
	Name string `json:"name"`
	Tag  string `json:"tag"`
}

func main() {
	fmt.Println("Starting API...")

	clusterConfig, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	clientSet, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		panic(err.Error())
	}

	router := gin.Default()
	router.POST("/rollout", func(c *gin.Context) {
		body := UpdateDeploymentRequest{}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		deployments, err := clientSet.AppsV1().Deployments("default").List(context.Background(), v1.ListOptions{Limit: 50})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		for _, deployment := range deployments.Items {
			imageName := strings.Split(deployment.Spec.Template.Spec.Containers[0].Image, ":")[0]
			log.Println("Image name: ", imageName)

			if imageName == body.Name {
				log.Println("Updating deployment: ", deployment.Name, " with image: ", imageName+":"+body.Tag)
				deployment.Spec.Template.Spec.Containers[0].Image = imageName + ":" + body.Tag
				_, err := clientSet.AppsV1().Deployments("default").Update(context.Background(), &deployment, v1.UpdateOptions{})
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
			}
		}

		c.JSON(http.StatusAccepted, body)
	})

	server := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			panic(err.Error())
		}
	}()

	quit := make(chan os.Signal)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down Server ...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatal("Server Shutdown:", err)
	}
	select {
	case <-ctx.Done():
		log.Println("timeout of 5 seconds.")
	}
	log.Println("Server exiting")
}
