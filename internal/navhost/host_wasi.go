//go:build wasip1

// Package navhost implements only the Navidrome JSON host ABI used by this plugin.
// It is an independently written MIT adapter, not the Navidrome Go SDK.
package navhost

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/extism/go-pdk"
)

// Host imports use Extism memory handles carrying UTF-8 JSON.
//
//go:wasmimport extism:host/user config_get
func configCall(uint64) uint64

//go:wasmimport extism:host/user storage_getstoragepath
func storageCall(uint64) uint64

//go:wasmimport extism:host/user users_getusers
func usersCall(uint64) uint64

//go:wasmimport extism:host/user library_getalllibraries
func librariesCall(uint64) uint64

//go:wasmimport extism:host/user library_getlibrary
func libraryCall(uint64) uint64

//go:wasmimport extism:host/user scheduler_schedulerecurring
func scheduleCall(uint64) uint64

//go:wasmimport extism:host/user subsonicapi_call
func subsonicCall(uint64) uint64

//go:wasmimport extism:host/user http_send
func httpCall(uint64) uint64

type operation uint8

const (
	opConfig operation = iota
	opStorage
	opUsers
	opLibraries
	opLibrary
	opSchedule
	opSubsonic
	opHTTP
)

func exchange(op operation, input, output any) error {
	encoded, err := json.Marshal(input)
	if err != nil {
		return err
	}
	block := pdk.AllocateBytes(encoded)
	defer block.Free()
	var offset uint64
	switch op {
	case opConfig:
		offset = configCall(block.Offset())
	case opStorage:
		offset = storageCall(block.Offset())
	case opUsers:
		offset = usersCall(block.Offset())
	case opLibraries:
		offset = librariesCall(block.Offset())
	case opLibrary:
		offset = libraryCall(block.Offset())
	case opSchedule:
		offset = scheduleCall(block.Offset())
	case opSubsonic:
		offset = subsonicCall(block.Offset())
	case opHTTP:
		offset = httpCall(block.Offset())
	}
	if offset == 0 {
		return errors.New("host returned no JSON response")
	}
	responseBlock := pdk.FindMemory(offset)
	return json.Unmarshal(responseBlock.ReadBytes(), output)
}
func result[T any](op operation, input any) (T, error) {
	var reply struct {
		Result T      `json:"result"`
		Error  string `json:"error"`
	}
	err := exchange(op, input, &reply)
	if err != nil {
		return reply.Result, err
	}
	if reply.Error != "" {
		return reply.Result, errors.New(reply.Error)
	}
	return reply.Result, nil
}
func ConfigValue(key string) (string, bool) {
	var reply struct {
		Value  string `json:"value"`
		Exists bool   `json:"exists"`
	}
	if exchange(opConfig, map[string]string{"key": key}, &reply) != nil {
		return "", false
	}
	return reply.Value, reply.Exists
}
func StorageRoot() (string, error) {
	value, err := result[string](opStorage, struct{}{})
	if err != nil {
		return "", err
	}
	if value != "/storage" {
		return "", fmt.Errorf("persistent plugin storage was not granted")
	}
	return value, nil
}

type User struct {
	UserName string `json:"userName"`
}
type LibraryInfo struct {
	ID         int32  `json:"id"`
	MountPoint string `json:"mountPoint"`
}

func Users() ([]User, error)            { return result[[]User](opUsers, struct{}{}) }
func Libraries() ([]LibraryInfo, error) { return result[[]LibraryInfo](opLibraries, struct{}{}) }
func Library(id int32) (LibraryInfo, error) {
	return result[LibraryInfo](opLibrary, map[string]int32{"id": id})
}
func Schedule(cron, payload, id string) error {
	var reply struct {
		Error string `json:"error"`
	}
	if err := exchange(opSchedule, map[string]string{"cronExpression": cron, "payload": payload, "scheduleId": id}, &reply); err != nil {
		return err
	}
	if reply.Error != "" {
		return errors.New(reply.Error)
	}
	return nil
}
func Subsonic(uri string) (string, error) {
	var reply struct {
		JSON  string `json:"responseJson"`
		Error string `json:"error"`
	}
	if err := exchange(opSubsonic, map[string]string{"uri": uri}, &reply); err != nil {
		return "", err
	}
	if reply.Error != "" {
		return "", errors.New(reply.Error)
	}
	return reply.JSON, nil
}

type Request struct {
	Method            string            `json:"method"`
	URL               string            `json:"url"`
	Headers           map[string]string `json:"headers"`
	Body              []byte            `json:"body,omitempty"`
	TimeoutMs         int32             `json:"timeoutMs"`
	NoFollowRedirects bool              `json:"noFollowRedirects"`
}
type Response struct {
	StatusCode int32             `json:"statusCode"`
	Headers    map[string]string `json:"headers"`
	Body       []byte            `json:"body"`
}

func Do(request Request) (*Response, error) {
	return result[*Response](opHTTP, struct {
		Request Request `json:"request"`
	}{request})
}
