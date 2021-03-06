package couchbasearray

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/coreos/go-etcd/etcd"
)

var SchedulerStateEmpty = ""
var SchedulerStateNew = "new"
var SchedulerStateRelax = "relax"
var SchedulerStateClustered = "clustered"
var SchedulerStateDeleted = "deleted"

var client *etcd.Client

func init() {
	client = NewEtcdClient()
}

func Schedule(path string) (map[string]NodeState, error) {
	announcements, err := GetClusterAnnouncements(path)
	if err != nil {
		return nil, err
	}

	currentStates, err := GetClusterStates(path)
	if err != nil {
		return nil, err
	}

	currentStates = ScheduleCore(announcements, currentStates)
	return SelectMaster(currentStates), nil
}

func ScheduleCore(announcements map[string]NodeState, currentStates map[string]NodeState) map[string]NodeState {
	for key, value := range announcements {
		if state, ok := currentStates[key]; ok {
			if state.SessionID == value.SessionID {
				if state.DesiredState == SchedulerStateNew && state.State == SchedulerStateNew {
					state.DesiredState = SchedulerStateClustered
					currentStates[key] = state
				}
			} else {
				log.Println("Resetting node")
				state.DesiredState = SchedulerStateNew
				state.State = ""
				state.SessionID = value.SessionID
				currentStates[key] = state
			}
		} else {
			log.Println("Unabled to find state for node ", key)
			currentStates[key] = NodeState{value.IPAddress, value.SessionID, false, "", SchedulerStateNew}
		}
	}

	for key := range currentStates {
		if _, ok := announcements[key]; ok {
			continue
		} else {
			delete(currentStates, key)
		}
	}

	return currentStates
}

func SelectMaster(currentStates map[string]NodeState) map[string]NodeState {
	if len(currentStates) == 0 {
		return currentStates
	}

	var lastKey string
	for key, state := range currentStates {
		if state.Master {
			return currentStates
		}

		lastKey = key
	}

	state := currentStates[lastKey]
	state.Master = true
	currentStates[lastKey] = state
	return currentStates
}

func GetClusterStates(base string) (map[string]NodeState, error) {
	values := make(map[string]NodeState)
	key := fmt.Sprintf("%s/states/", base)
	response, err := client.Get(key, false, false)
	if err != nil {
		if strings.Contains(err.Error(), "Key not found") {
			return values, nil
		}
		return nil, err
	}

	for _, node := range response.Node.Nodes {
		var state NodeState
		err = json.Unmarshal([]byte(node.Value), &state)
		if err != nil {
			return nil, err
		}

		sections := strings.Split(node.Key, "/")
		nodeKey := sections[len(sections)-1]
		values[nodeKey] = state
		log.Println("Loaded state ", state)
	}

	return values, nil
}

func SaveClusterStates(base string, states map[string]NodeState) error {
	for _, stateValue := range states {
		bytes, err := json.Marshal(stateValue)
		key := fmt.Sprintf("%s/states/%s", base, stateValue.IPAddress)
		log.Println("Saving State ", stateValue)
		_, err = client.Set(key, string(bytes), 0)
		if err != nil {
			return err
		}
	}

	return nil
}

func ClearClusterStates(base string) error {
	key := fmt.Sprintf("%s/states/", base)
	_, err := client.Delete(key, true)
	if err != nil {
		if strings.Contains(err.Error(), "Key not found") {
			return nil
		}
	}

	return err
}

func ClearAnnouncments(base string) error {
	key := fmt.Sprintf("%s/announcements/", base)
	_, err := client.Delete(key, true)
	if err != nil {
		if strings.Contains(err.Error(), "Key not found") {
			return nil
		}
	}

	return err
}

func GetClusterAnnouncements(path string) (map[string]NodeState, error) {
	values := make(map[string]NodeState)
	key := fmt.Sprintf("%s/announcements/", path)
	response, err := client.Get(key, false, false)
	if err != nil {
		if strings.Contains(err.Error(), "Key not found") {
			return values, nil
		}
		return nil, err
	}

	for _, node := range response.Node.Nodes {
		var state NodeState
		err = json.Unmarshal([]byte(node.Value), &state)
		if err != nil {
			return nil, err
		}

		sections := strings.Split(node.Key, "/")
		nodeKey := sections[len(sections)-1]
		values[nodeKey] = state
		log.Println("Loaded announcement ", state)
	}

	return values, nil
}

func SetClusterAnnouncement(base string, state NodeState) error {
	path := fmt.Sprintf("%s/announcements/%s", base, state.IPAddress)
	bytes, err := json.Marshal(state)
	if err != nil {
		return err
	}
	if _, err := client.Set(path, string(bytes), 10); err != nil {
		return err
	}

	return nil
}

type NodeState struct {
	IPAddress    string `json:"ipAddress"`
	SessionID    string `json:"sessionID"`
	Master       bool   `json:"master"`
	State        string `json:"state"`
	DesiredState string `json:"desiredState"`
}

func NewEtcdClient() (client *etcd.Client) {
	var etcdClient *etcd.Client
	peersStr := os.Getenv("ETCDCTL_PEERS")
	if len(peersStr) > 0 {
		log.Println("Connecting to etcd peers : " + peersStr)
		peers := strings.Split(peersStr, ",")
		etcdClient = etcd.NewClient(peers)
	} else {
		etcdClient = etcd.NewClient(nil)
	}

	return etcdClient
}
