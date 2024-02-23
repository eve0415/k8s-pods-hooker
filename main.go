package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	v1Meta "k8s.io/apimachinery/pkg/apis/meta/v1"
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

type UpdateDeploymentRequestBody struct {
	NameSpace string `json:"namespace"`
	ImageName string `json:"name" binding:"required"`
	Tag       string `json:"tag" binding:"required"`
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
		body := UpdateDeploymentRequestBody{}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if body.NameSpace == "" {
			body.NameSpace = "default"
		}

		deployments, err := clientSet.AppsV1().Deployments(body.NameSpace).List(context.Background(), v1Meta.ListOptions{Limit: 50})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		var found = false
		for _, deployment := range deployments.Items {
			imageName := strings.Split(deployment.Spec.Template.Spec.Containers[0].Image, ":")[0]
			if imageName == body.ImageName {
				found = true
				log.Println("Updating deployment:", deployment.Name, "with image:", imageName+":"+body.Tag)
				deployment.Spec.Template.Spec.Containers[0].Image = imageName + ":" + body.Tag
				_, err := clientSet.AppsV1().Deployments(body.NameSpace).Update(context.Background(), &deployment, v1Meta.UpdateOptions{})
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
				break
			}
		}

		if found == false {
			c.JSON(http.StatusNotFound, gin.H{"error": "No deployment found with image name: " + body.ImageName})
			return
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
		log.Fatal("Server Shutdown Error:", err)
	}
	select {
	case <-ctx.Done():
		log.Println("Timeout of 5 seconds.")
	}
	log.Println("Server exiting")
}
