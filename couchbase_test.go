package couchbasearray

import (
	"encoding/json"
	"fmt"
	"log"
	"testing"

	"code.google.com/p/go-uuid/uuid"
)

func TestClusterScenarios(t *testing.T) {
	path := "/TestClusterInitialization"
	if err := ClearClusterStates(path); err != nil {
		t.Fatal(err)
	}
	if err := ClearAnnouncments(path); err != nil {
		t.Fatal(err)
	}
	//
	//	First cluster boostrap
	//
	_, err := CreateTestNodes(path, 2)
	if err != nil {
		t.Fatal(err)
	}

	currentStates, err := Schedule(path)
	if err != nil {
		t.Fatal(err)
	}

	log.Println("Current States")
	log.Println(currentStates)

	log.Println(currentStates)
	for _, state := range currentStates {
		if state.DesiredState != SchedulerStateNew {
			t.Fatal("Expected state should be 'new'")
		}

		if state.State != SchedulerStateEmpty {
			t.Fatal("Expected state should be ''")
		}
	}
	//
	// Set status to clustered
	//
	masterFound := false
	for key, state := range currentStates {
		state.State = state.DesiredState
		currentStates[key] = state
		if !masterFound && state.Master {
			masterFound = true
		}
	}

	if !masterFound {
		t.Fatal("Expected a master to be selected")
	}

	SaveClusterStates(path, currentStates)

	currentStates, err = Schedule(path)
	if err != nil {
		t.Fatal(err)
	}

	log.Println("Current States")
	log.Println(currentStates)

	log.Println(currentStates)
	for _, state := range currentStates {
		if state.DesiredState != SchedulerStateClustered {
			t.Fatal("Expected state should be 'clustered'")
		}

		if state.State != SchedulerStateNew {
			t.Fatal("Expected state should be 'new'")
		}
	}
	//
	//	Simulate non master machine reboot
	//
	var nonMasterKey string
	for key, state := range currentStates {
		state.State = state.DesiredState
		if !state.Master {
			state.SessionID = uuid.New()
			nonMasterKey = key
		}

		currentStates[key] = state
	}

	SaveClusterStates(path, currentStates)

	currentStates, err = Schedule(path)
	if err != nil {
		t.Fatal(err)
	}

	log.Println("Current States")
	log.Println(currentStates)

	log.Println(currentStates)
	for key, state := range currentStates {
		if state.State != SchedulerStateClustered {
			if key == nonMasterKey {
				if state.State != SchedulerStateEmpty {
					t.Fatal("Expected state should be ''")
				}

				if state.DesiredState != SchedulerStateNew {
					t.Fatal("Expected state should be 'new'")
				}

				if state.Master {
					t.Fatal("Expected state should not be 'mast'")
				}
			} else {
				t.Fatal("Expected state should be 'clustered'")
			}
		}
	}
	//
	//	Simulate master machine reboot
	//
	var masterKey string
	for key, state := range currentStates {
		state.State = SchedulerStateClustered
		if !state.Master {
			state.SessionID = uuid.New()
			masterKey = key
		}

		currentStates[key] = state
	}

	SaveClusterStates(path, currentStates)

	currentStates, err = Schedule(path)
	if err != nil {
		t.Fatal(err)
	}

	log.Println("Current States")
	log.Println(currentStates)

	log.Println(currentStates)
	for key, state := range currentStates {
		if state.State != SchedulerStateClustered {
			if key == masterKey {
				if state.State != SchedulerStateEmpty {
					t.Fatal("Expected state should be ''")
				}

				if state.DesiredState != SchedulerStateNew {
					t.Fatal("Expected state should be 'new'")
				}

				if state.Master {
					t.Fatal("Expected state should not be 'mast'")
				}
			} else {
				t.Fatal("Expected state should be 'clustered'")
			}
		} else {
			if !state.Master {
				t.Fatal("Expected state should be 'master'")
			}

			if state.DesiredState != SchedulerStateClustered {
				t.Fatal("Expected state should be 'Clustered'")
			}
		}
	}
}

func TestGetClusterAnnouncements(t *testing.T) {
	path := "/TestGetClusterAnnouncements"
	testNodes, err := CreateTestNodes(path, 2)
	if err != nil {
		t.Fatal(err)
	}

	nodes, err := GetClusterAnnouncements(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(nodes) != len(testNodes) {
		t.Fatal("Difference in result lengths")
	}
}

func CreateTestNodes(base string, count int) (map[string]NodeState, error) {
	client := NewEtcdClient()
	values := make(map[string]NodeState)
	for i := 0; i < count; i++ {
		ip := fmt.Sprintf("10.100.2.%v", i)
		path := fmt.Sprintf("%s/announcements/%s", base, ip)
		id := uuid.New()
		node := NodeState{ip, id, false, "", ""}
		values[ip] = node
		bytes, err := json.Marshal(node)
		if err != nil {
			return nil, err
		}
		if _, err := client.Set(path, string(bytes), 0); err != nil {
			return nil, err
		}
	}

	return values, nil
}
