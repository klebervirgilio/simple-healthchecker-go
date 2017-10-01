package healthcheck

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-redis/redis"
	"gopkg.in/mgo.v2"
)

// Status represents the service status
type Status struct {
	Status  bool
	Message string
}

// Healthcheck our class
type Healthcheck struct {
	Name string
}

// MongoHealthcheck - composes Healthcheck
type MongoHealthcheck struct {
	Healthcheck
}

// RedisHealthcheck - composes Healthcheck
type RedisHealthcheck struct {
	Healthcheck
}

// an instance method
func (h Healthcheck) timeout(timeout int, fn func()) error {
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

// NewMongoHealthcheck is the constructor method of MongoHealthcheck
func NewMongoHealthcheck() MongoHealthcheck {
	return MongoHealthcheck{Healthcheck{"Mongo"}}
}

// NewRedisHealthcheck is the constructor method of RedisHealthcheck
func NewRedisHealthcheck() RedisHealthcheck {
	return RedisHealthcheck{Healthcheck{"Redis"}}
}

// Statuser if it walks like a Statuser, it is a Statuser.
type Statuser interface {
	Status() (Status, error)
}

// Status - implementing Statuser on MongoHealthcheck
func (m MongoHealthcheck) Status() (Status, error) {
	var mongoErr error
	var session *mgo.Session

	mongoHost := os.Getenv("MONGO_HOST")
	timeout, _ := strconv.Atoi(os.Getenv("MONGO_TIMEOUT"))
	wait, _ := strconv.Atoi(os.Getenv("WAIT"))

	err := m.timeout(timeout, func() {
		session, mongoErr = mgo.Dial(mongoHost)
		if mongoErr != nil {
			return
		}
		time.Sleep(time.Duration(wait) * time.Second)
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

// Status - implementing Statuser
func (r RedisHealthcheck) Status() (Status, error) {
	redisHost := os.Getenv("REDIS_HOST")

	client := redis.NewClient(&redis.Options{Addr: redisHost})
	defer client.Close()

	timeout, _ := strconv.Atoi(os.Getenv("REDIS_TIMEOUT"))
	wait, _ := strconv.Atoi(os.Getenv("WAIT"))

	var redisErr error
	err := r.timeout(timeout, func() {
		time.Sleep(time.Duration(wait) * time.Second)
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

// not exported methods - private. healthcheck runner
func runHealthChekers(services []Statuser) (*Status, error) {
	for _, service := range services {
		status, err := service.Status()
		if err != nil {
			return &status, err
		}
	}
	return nil, nil
}

// healthcheck runner, in parallel
func runHealthChekersParallel(unhealth, health chan Status, services []Statuser) {
	for _, service := range services {
		go func(service Statuser) {
			status, err := service.Status()
			if err != nil {
				unhealth <- status
				return
			}
			health <- status
		}(service)
	}
}

func getVitalServices() []Statuser {
	rHck := NewRedisHealthcheck()
	mHck := NewMongoHealthcheck()

	return []Statuser{rHck, mHck}
}

func duration(fn func()) time.Duration {
	startTime := time.Now().UTC()
	fn()
	duration := time.Since(startTime) / time.Millisecond
	return duration
}

// Handler a serial healthcheck
func Handler(w http.ResponseWriter, r *http.Request) {
	var status *Status
	var err error

	services := getVitalServices()

	duration := duration(func() {
		status, err = runHealthChekers(services)
	})

	if err == nil {
		fmt.Fprintf(w, "WORKING %d ms", duration)
		return
	}

	fmt.Fprintf(w, "%s %d ms", status.Message, duration)
}

// ParallelHandler a parallel healthcheck
func ParallelHandler(w http.ResponseWriter, r *http.Request) {
	unhealth := make(chan Status, 1)
	health := make(chan Status)
	checkSum := 0
	startTime := time.Now().UTC()

	services := getVitalServices()
	runHealthChekersParallel(unhealth, health, services)

	for {
		select {
		case status := <-unhealth:
			duration := time.Since(startTime) / time.Millisecond
			fmt.Fprintf(w, "%s - %d ms", status.Message, duration)
			return
		case <-health:
			if checkSum++; checkSum == len(services) {
				duration := time.Since(startTime) / time.Millisecond
				fmt.Fprintf(w, "WORKING %d ms", duration)
				return
			}
		}
	}
}
