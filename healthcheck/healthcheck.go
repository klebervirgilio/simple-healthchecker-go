package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-redis/redis"
	"gopkg.in/mgo.v2"
)

// Statuser if it walks like a Statuser, it is a Healthchecker.
type Statuser interface {
	Status() (Status, error)
}

type Healthchecker struct {
	Name string
}

func (h Healthchecker) timeout(timeout int, fn func()) error {
	ch := make(chan bool, 1)
	go func() {
		fn()
		ch <- true
	}()

	select {
	case <-ch:
	case <-time.After(time.Duration(timeout) * time.Second):
		return fmt.Errorf("%s Timeout", h.Name)
	}

	return nil
}

type MongoHealthchecker struct {
	Healthchecker
}

type RedisHealthchecker struct {
	Healthchecker
}

func NewMongoHealthchecker() MongoHealthchecker {
	return MongoHealthchecker{Healthchecker{"Mongo"}}
}

func NewRedisHealthchecker() RedisHealthchecker {
	return RedisHealthchecker{Healthchecker{"Redis"}}
}

type Status struct {
	Status  bool
	Message string
}

func (m MongoHealthchecker) Status() (Status, error) {
	var mongoErr error
	var session *mgo.Session

	err := m.timeout(2, func() {
		session, mongoErr = mgo.Dial("mongodb://localhost:27017")
		if mongoErr != nil {
			return
		}
		mongoErr = session.Ping()
	})

	if err != nil {
		return Status{false, fmt.Sprintf("%s - %s", m.Name, err.Error())}, err
	}

	if mongoErr != nil {
		return Status{false, fmt.Sprintf("%s - %s", m.Name, mongoErr.Error())}, mongoErr
	}

	defer session.Close()
	return Status{true, ""}, nil
}

func (r RedisHealthchecker) Status() (Status, error) {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer client.Close()

	var redisErr error
	err := r.timeout(2, func() {
		_, redisErr = client.Ping().Result()
	})

	if err != nil {
		return Status{false, fmt.Sprintf("%s - %s", r.Name, err.Error())}, err
	}

	if redisErr != nil {
		return Status{false, fmt.Sprintf("%s - %s", r.Name, redisErr.Error())}, redisErr
	}

	return Status{true, ""}, nil
}

func runHealthChekers(unhealth chan Status, services []Statuser) {
	var err error
	var status Status

	for _, service := range services {
		status, err = service.Status()

		if err != nil {
			unhealth <- status
			break
		}
	}
}

func runHealthChekersParallel(unhealth chan Status, services []Statuser) {
	var err error
	var status Status

	for _, service := range services {
		go func(service Statuser) {
			status, err = service.Status()
			if err != nil {
				unhealth <- status
			}
		}(service)
	}
}

func healthcheck(w http.ResponseWriter, r *http.Request) {
	unhealth := make(chan Status, 1)

	rHck := NewRedisHealthchecker()
	mHck := NewMongoHealthchecker()

	runHealthChekers(unhealth, []Statuser{rHck, mHck})

	select {
	case status := <-unhealth:
		fmt.Fprintf(w, status.Message)
	default:
		fmt.Fprintf(w, "WORKING")
	}
}

func healthcheckParallel(w http.ResponseWriter, r *http.Request) {
	unhealth := make(chan Status, 1)

	rHck := NewRedisHealthchecker()
	mHck := NewMongoHealthchecker()

	runHealthChekersParallel(unhealth, []Statuser{rHck, mHck})

	select {
	case status := <-unhealth:
		fmt.Fprintf(w, status.Message)
	default:
		fmt.Fprintf(w, "WORKING")
	}
}

func main() {
	fmt.Println("Listening on port 5555")
	http.HandleFunc("/healthcheck", healthcheck)
	http.HandleFunc("/healthcheck-parallel", healthcheckParallel)
	http.ListenAndServe(":5555", nil)
}
