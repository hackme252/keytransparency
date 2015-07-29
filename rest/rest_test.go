// Copyright 2015 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/google/e2e-key-server/rest/handlers"

	v2pb "github.com/google/e2e-key-server/proto/v2"
	context "golang.org/x/net/context"
)

const (
	valid_ts            = "2015-05-18T23:58:36.000Z"
	invalid_ts          = "Mon May 18 23:58:36 UTC 2015"
	ts_seconds          = 1431993516
	primary_test_email  = "e2eshare.test@gmail.com"
	primary_test_app_id = "gmail"
	primary_test_key_id = "mykey"
)

type fakeJSONParserReader struct {
	*bytes.Buffer
}

func (pr fakeJSONParserReader) Close() error {
	return nil
}

type FakeServer struct {
}

func Fake_Handler(srv interface{}, ctx context.Context, w http.ResponseWriter, r *http.Request, info *handlers.HandlerInfo) error {
	w.Write([]byte("hi"))
	return nil
}

func Fake_Initializer(rInfo handlers.RouteInfo) *handlers.HandlerInfo {
	return nil
}

func Fake_RequestHandler(srv interface{}, ctx context.Context, arg interface{}) (*interface{}, error) {
	b := true
	i := new(interface{})
	*i = b
	return i, nil
}

func TestFoo(t *testing.T) {
	v1 := &FakeServer{}
	s := New(v1)
	rInfo := handlers.RouteInfo{
		"/hi",
		-1,
		-1,
		"GET",
		Fake_Initializer,
		Fake_RequestHandler,
	}
	s.AddHandler(rInfo, Fake_Handler)

	server := httptest.NewServer(s.Handlers())
	defer server.Close()
	res, err := http.Get(fmt.Sprintf("%s/hi", server.URL))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := res.StatusCode, http.StatusOK; got != want {
		t.Errorf("GET: %v = %v, want %v", res.Request.URL, got, want)
	}
}

func TestGetUser_InitiateHandlerInfo(t *testing.T) {
	var tests = []struct {
		path         string
		userId       string
		appId        string
		tm           string
		isOutTimeNil bool
		parserNilErr bool
	}{
		{"/v1/users/" + primary_test_email + "?app_id=" + primary_test_app_id +
			"&time=" + valid_ts,
			primary_test_email, primary_test_app_id, valid_ts, false, true},
		{"/v1/users/" + primary_test_email + "?time=" + valid_ts,
			primary_test_email, "", valid_ts, false, true},
		{"/v1/users/" + primary_test_email + "?app_id=" + primary_test_app_id,
			primary_test_email, primary_test_app_id, "", true, true},
		{"/v1/users/" + primary_test_email,
			primary_test_email, "", "", true, true},
		{"/v1/users/" + primary_test_email + "?app_id=" + primary_test_app_id +
			"&time=" + invalid_ts,
			primary_test_email, primary_test_app_id, invalid_ts, false, false},
	}

	for _, test := range tests {
		rInfo := handlers.RouteInfo{
			test.path,
			2,
			-1,
			"GET",
			Fake_Initializer,
			Fake_RequestHandler,
		}
		// Body is empty when invoking get user API.
		jsonBody := "{}"

		info := GetUser_InitializeHandlerInfo(rInfo)

		if _, ok := info.Arg.(*v2pb.GetUserRequest); !ok {
			t.Errorf("info.Arg is not of type v2pb.GetUserRequest")
		}

		r, _ := http.NewRequest(rInfo.Method, rInfo.Path, fakeJSONParserReader{bytes.NewBufferString(jsonBody)})
		err := info.Parser(r, &info.Arg)
		if got, want := (err == nil), test.parserNilErr; got != want {
			t.Errorf("Unexpected err = (%v), want nil = %v", err, test.parserNilErr)
		}
		// If there's an error parsing, the test cannot be completed.
		// The parsing error might be expected though.
		if err != nil {
			continue
		}

		// Call JSONDecoder to simulate decoding JSON -> Proto.
		err = JSONDecoder(r, &info.Arg)
		if err != nil {
			t.Errorf("Error while calling JSONDecoder, this should not happen. err: %v", err)
		}

		if got, want := info.Arg.(*v2pb.GetUserRequest).UserId, test.userId; got != want {
			t.Errorf("UserId = %v, want %v", got, want)
		}
		if got, want := info.Arg.(*v2pb.GetUserRequest).AppId, test.appId; got != want {
			t.Errorf("AppId = %v, want %v", got, want)
		}
		if got, want := info.Arg.(*v2pb.GetUserRequest).Time == nil, test.isOutTimeNil; got != want {
			t.Errorf("Unexpected time = (%v), want nil = %v", info.Arg.(*v2pb.GetUserRequest).Time, want)
		}
		if test.isOutTimeNil == false {
			tm, err := time.Parse(time.RFC3339, test.tm)
			if err != nil {
				t.Errorf("Error while parsing time, this should not happen.")
			}
			if gots, gotn, wants, wantn := info.Arg.(*v2pb.GetUserRequest).GetTime().Seconds, info.Arg.(*v2pb.GetUserRequest).GetTime().Nanos, tm.Unix(), tm.Nanosecond(); gots != wants || gotn != int32(wantn) {
				t.Errorf("Time = %v [sec] %v [nsec], want %v [sec] %v [nsec]", gots, gotn, wants, wantn)
			}
		}

		v1 := &FakeServer{}
		srv := New(v1)
		resp, err := info.H(srv, nil, nil)
		if err != nil {
			t.Errorf("Error while calling Fake_RequestHandler, this should not happen.")
		}
		if got, want := (*resp).(bool), true; got != want {
			t.Errorf("resp = %v, want %v.", got, want)
		}
	}
}

func TestCreateKey_InitiateHandlerInfo(t *testing.T) {
	var tests = []struct {
		path         string
		userId       string
		userIdIndex  int
		tm           string
		isTimeNil    bool
		jsonBody     string
		parserNilErr bool
	}{
		{"/v1/users/" + primary_test_email + "/keys", primary_test_email, 2,
			valid_ts, false,
			"{\"signed_key\":{\"key\": {\"creation_time\": \"" + valid_ts + "\"}}}",
			true},
		{"/v1/users/" + primary_test_email + "/keys", primary_test_email, 4,
			valid_ts, false,
			"{\"signed_key\":{\"key\": {\"creation_time\": \"" + valid_ts + "\"}}}",
			false},
		{"/v1/users/" + primary_test_email + "/keys", primary_test_email, -1,
			valid_ts, false,
			"{\"signed_key\":{\"key\": {\"creation_time\": \"" + valid_ts + "\"}}}",
			false},
		{"/v1/users/" + primary_test_email + "/keys", primary_test_email, 2,
			valid_ts, true,
			"{\"signed_key\":{\"key\": {\"creation_time\": \"\"}}}",
			false},
		{"/v1/users/" + primary_test_email + "/keys", primary_test_email, 2,
			valid_ts, true,
			"{}",
			true},
	}
	for _, test := range tests {
		rInfo := handlers.RouteInfo{
			test.path,
			test.userIdIndex,
			-1,
			"POST",
			Fake_Initializer,
			Fake_RequestHandler,
		}

		info := CreateKey_InitializeHandlerInfo(rInfo)

		if _, ok := info.Arg.(*v2pb.CreateKeyRequest); !ok {
			t.Errorf("info.Arg is not of type v2pb.CreateKeyRequest")
		}

		r, _ := http.NewRequest(rInfo.Method, rInfo.Path, fakeJSONParserReader{bytes.NewBufferString(test.jsonBody)})
		err := info.Parser(r, &info.Arg)
		if got, want := (err == nil), test.parserNilErr; got != want {
			t.Errorf("Unexpected err = (%v), want nil = %v", err, test.parserNilErr)
		}
		// If there's an error parsing, the test cannot be completed.
		// The parsing error might be expected though.
		if err != nil {
			continue
		}

		// Call JSONDecoder to simulate decoding JSON -> Proto.
		err = JSONDecoder(r, &info.Arg)
		if err != nil {
			t.Errorf("Error while calling JSONDecoder, this should not happen. err: %v", err)
		}

		if got, want := info.Arg.(*v2pb.CreateKeyRequest).UserId, test.userId; got != want {
			t.Errorf("UserId = %v, want %v", got, want)
		}
		if test.isTimeNil == false {
			tm, err := time.Parse(time.RFC3339, test.tm)
			if err != nil {
				t.Errorf("Error while parsing time, this should not happen.")
			}
			if gots, gotn, wants, wantn := info.Arg.(*v2pb.CreateKeyRequest).GetSignedKey().GetKey().GetCreationTime().Seconds, info.Arg.(*v2pb.CreateKeyRequest).GetSignedKey().GetKey().GetCreationTime().Nanos, tm.Unix(), tm.Nanosecond(); gots != wants || gotn != int32(wantn) {
				t.Errorf("Time = %v [sec] %v [nsec], want %v [sec] %v [nsec]", gots, gotn, wants, wantn)
			}
		}

		v1 := &FakeServer{}
		srv := New(v1)
		resp, err := info.H(srv, nil, nil)
		if err != nil {
			t.Errorf("Error while calling Fake_RequestHandler, this should not happen.")
		}
		if got, want := (*resp).(bool), true; got != want {
			t.Errorf("resp = %v, want %v.", got, want)
		}
	}
}

func TestUpdateKey_InitiateHandlerInfo(t *testing.T) {
	var tests = []struct {
		path         string
		userId       string
		userIdIndex  int
		keyId        string
		keyIdIndex   int
		tm           string
		isTimeNil    bool
		jsonBody     string
		parserNilErr bool
	}{
		{"/v1/users/" + primary_test_email + "/keys/" + primary_test_key_id,
			primary_test_email, 2, primary_test_key_id, 4, valid_ts, false,
			"{\"signed_key\":{\"key\": {\"creation_time\": \"" + valid_ts + "\"}}}",
			true},
		{"/v1/users/" + primary_test_email + "/keys/" + primary_test_key_id,
			primary_test_email, 2, primary_test_key_id, 5, valid_ts, false,
			"{\"signed_key\":{\"key\": {\"creation_time\": \"" + valid_ts + "\"}}}",
			false},
		{"/v1/users/" + primary_test_email + "/keys/" + primary_test_key_id,
			primary_test_email, 2, primary_test_key_id, -1, valid_ts, false,
			"{\"signed_key\":{\"key\": {\"creation_time\": \"" + valid_ts + "\"}}}",
			false},
		{"/v1/users/" + primary_test_email + "/keys/" + primary_test_key_id,
			primary_test_email, 2, primary_test_key_id, 4, valid_ts, true,
			"{\"signed_key\":{\"key\": {\"creation_time\": \"\"}}}",
			false},
		{"/v1/users/" + primary_test_email + "/keys/" + primary_test_key_id,
			primary_test_email, 2, primary_test_key_id, 4, valid_ts, true,
			"{}",
			true},
	}
	for _, test := range tests {
		rInfo := handlers.RouteInfo{
			test.path,
			test.userIdIndex,
			test.keyIdIndex,
			"PUT",
			Fake_Initializer,
			Fake_RequestHandler,
		}

		info := UpdateKey_InitializeHandlerInfo(rInfo)

		if _, ok := info.Arg.(*v2pb.UpdateKeyRequest); !ok {
			t.Errorf("info.Arg is not of type v2pb.UpdateKeyRequest")
		}

		r, _ := http.NewRequest(rInfo.Method, rInfo.Path, fakeJSONParserReader{bytes.NewBufferString(test.jsonBody)})
		err := info.Parser(r, &info.Arg)
		if got, want := (err == nil), test.parserNilErr; got != want {
			t.Errorf("Unexpected err = (%v), want nil = %v", err, test.parserNilErr)
		}
		// If there's an error parsing, the test cannot be completed.
		// The parsing error might be expected though.
		if err != nil {
			continue
		}

		// Call JSONDecoder to simulate decoding JSON -> Proto.
		err = JSONDecoder(r, &info.Arg)
		if err != nil {
			t.Errorf("Error while calling JSONDecoder, this should not happen. err: %v", err)
		}

		if got, want := info.Arg.(*v2pb.UpdateKeyRequest).UserId, test.userId; got != want {
			t.Errorf("UserId = %v, want %v", got, want)
		}
		if got, want := info.Arg.(*v2pb.UpdateKeyRequest).KeyId, test.keyId; got != want {
			t.Errorf("KeyId = %v, want %v", got, want)
		}
		if test.isTimeNil == false {
			tm, err := time.Parse(time.RFC3339, test.tm)
			if err != nil {
				t.Errorf("Error while parsing time, this should not happen.")
			}
			if gots, gotn, wants, wantn := info.Arg.(*v2pb.UpdateKeyRequest).GetSignedKey().GetKey().GetCreationTime().Seconds, info.Arg.(*v2pb.UpdateKeyRequest).GetSignedKey().GetKey().GetCreationTime().Nanos, tm.Unix(), tm.Nanosecond(); gots != wants || gotn != int32(wantn) {
				t.Errorf("Time = %v [sec] %v [nsec], want %v [sec] %v [nsec]", gots, gotn, wants, wantn)
			}
		}

		v1 := &FakeServer{}
		srv := New(v1)
		resp, err := info.H(srv, nil, nil)
		if err != nil {
			t.Errorf("Error while calling Fake_RequestHandler, this should not happen.")
		}
		if got, want := (*resp).(bool), true; got != want {
			t.Errorf("resp = %v, want %v.", got, want)
		}
	}
}

func TestDeleteKey_InitiateHandlerInfo(t *testing.T) {
	var tests = []struct {
		path         string
		userId       string
		userIdIndex  int
		keyId        string
		keyIdIndex   int
		parserNilErr bool
	}{
		{"/v1/users/" + primary_test_email + "/keys/" + primary_test_key_id,
			primary_test_email, 2, primary_test_key_id, 4, true},
		{"/v1/users/" + primary_test_email + "/keys/" + primary_test_key_id,
			primary_test_email, 5, primary_test_key_id, 4, false},
		{"/v1/users/" + primary_test_email + "/keys/" + primary_test_key_id,
			primary_test_email, -1, primary_test_key_id, 4, false},
		{"/v1/users/" + primary_test_email + "/keys/" + primary_test_key_id,
			primary_test_email, 2, primary_test_key_id, 5, false},
		{"/v1/users/" + primary_test_email + "/keys/" + primary_test_key_id,
			primary_test_email, 2, primary_test_key_id, -1, false},
	}
	for _, test := range tests {
		rInfo := handlers.RouteInfo{
			test.path,
			test.userIdIndex,
			test.keyIdIndex,
			"DELETE",
			Fake_Initializer,
			Fake_RequestHandler,
		}
		// Body is empty when invoking delete key API.
		jsonBody := "{}"

		info := DeleteKey_InitializeHandlerInfo(rInfo)

		if _, ok := info.Arg.(*v2pb.DeleteKeyRequest); !ok {
			t.Errorf("info.Arg is not of type v2pb.DeleteKeyRequest")
		}

		r, _ := http.NewRequest(rInfo.Method, rInfo.Path, fakeJSONParserReader{bytes.NewBufferString(jsonBody)})
		err := info.Parser(r, &info.Arg)
		if got, want := (err == nil), test.parserNilErr; got != want {
			t.Errorf("Unexpected err = (%v), want nil = %v", err, test.parserNilErr)
		}
		// If there's an error parsing, the test cannot be completed.
		// The parsing error might be expected though.
		if err != nil {
			continue
		}

		// Call JSONDecoder to simulate decoding JSON -> Proto.
		err = JSONDecoder(r, &info.Arg)
		if err != nil {
			t.Errorf("Error while calling JSONDecoder, this should not happen. err: %v", err)
		}

		if got, want := info.Arg.(*v2pb.DeleteKeyRequest).UserId, test.userId; got != want {
			t.Errorf("UserId = %v, want %v", got, want)
		}
		if got, want := info.Arg.(*v2pb.DeleteKeyRequest).KeyId, test.keyId; got != want {
			t.Errorf("KeyId = %v, want %v", got, want)
		}

		v1 := &FakeServer{}
		srv := New(v1)
		resp, err := info.H(srv, nil, nil)
		if err != nil {
			t.Errorf("Error while calling Fake_RequestHandler, this should not happen.")
		}
		if got, want := (*resp).(bool), true; got != want {
			t.Errorf("resp = %v, want %v.", got, want)
		}
	}
}

func JSONDecoder(r *http.Request, v interface{}) error {
	decoder := json.NewDecoder(r.Body)
	return decoder.Decode(v)
}

func TestParseURLComponent(t *testing.T) {
	var tests = []struct {
		comp   []string
		index  int
		out    string
		nilErr bool
	}{
		{[]string{"v1", "users", primary_test_email}, 2, primary_test_email, true},
		{[]string{"v1", "users", "e2eshare.test@cs.ox.ac.uk"}, -1, "", false},
		{[]string{"v1", "users", "e2eshare.test@cs.ox.ac.uk"}, 3, "", false},
	}
	for _, test := range tests {
		gots, gote := parseURLComponent(test.comp, test.index)
		wants := test.out
		wante := test.nilErr
		if gots != wants || wante != (gote == nil) {
			t.Errorf("Error while parsing User ID. Input = (%v, %v), got ('%v', %v), want ('%v', nil = %v)", test.comp, test.index, gots, gote, wants, wante)
		}
	}
}

func TestParseJson(t *testing.T) {
	var tests = []struct {
		inJSON    string
		outJSON   string
		outNilErr bool
	}{
		// Empty string
		{"", "", true},
		// Basic cases.
		{"\"creation_time\": \"" + valid_ts + "\"",
			"\"creation_time\": {\"seconds\": " +
				strconv.Itoa(ts_seconds) + ", \"nanos\": 0}", true},
		{"{\"creation_time\": \"" + valid_ts + "\"}",
			"{\"creation_time\": {\"seconds\": " +
				strconv.Itoa(ts_seconds) + ", \"nanos\": 0}}", true},
		// Nested case.
		{"{\"signed_key\":{\"key\": {\"creation_time\": \"" + valid_ts + "\"}}}",
			"{\"signed_key\":{\"key\": {\"creation_time\": {\"seconds\": " +
				strconv.Itoa(ts_seconds) + ", \"nanos\": 0}}}}", true},
		// Nothing to be changed.
		{"nothing to be changed here", "nothing to be changed here", true},
		// Multiple keywords.
		{"\"creation_time\": \"" + valid_ts + "\", \"creation_time\": \"" +
			valid_ts + "\"",
			"\"creation_time\": {\"seconds\": " + strconv.Itoa(ts_seconds) +
				", \"nanos\": 0}, \"creation_time\": {\"seconds\": " +
				strconv.Itoa(ts_seconds) + ", \"nanos\": 0}", true},
		// Invalid timestamp.
		{"\"creation_time\": \"invalid\"", "\"creation_time\": \"invalid\"", false},
		// Empty timestamp.
		{"\"creation_time\": \"\"", "\"creation_time\": \"\"", false},
		{"\"creation_time\": \"\", \"creation_time\": \"\"",
			"\"creation_time\": \"\", \"creation_time\": \"\"", false},
		// Malformed JSON, missing " at the beginning of invalid
		// timestamp.
		{"\"creation_time\": invalid\"", "\"creation_time\": invalid\"", true},
		// Malformed JSON, missing " at the end of invalid timestamp.
		{"\"creation_time\": \"invalid", "\"creation_time\": \"invalid", true},
		// Malformed JSON, missing " at the beginning and end of
		// invalid timestamp.
		{"\"creation_time\": invalid", "\"creation_time\": invalid", true},
		// Malformed JSON, missing " at the end of valid timestamp.
		{"\"creation_time\": \"" + valid_ts, "\"creation_time\": \"" + valid_ts, true},
		// keyword is not surrounded by "", in four cases: invalid
		// timestamp, basic, nested and multiple keywords.
		{"creation_time: \"invalid\"", "creation_time: \"invalid\"", false},
		{"{creation_time: \"" + valid_ts + "\"}",
			"{creation_time: {\"seconds\": " +
				strconv.Itoa(ts_seconds) + ", \"nanos\": 0}}", true},
		{"{\"signed_key\":{\"key\": {creation_time: \"" + valid_ts + "\"}}}",
			"{\"signed_key\":{\"key\": {creation_time: {\"seconds\": " +
				strconv.Itoa(ts_seconds) + ", \"nanos\": 0}}}}", true},
		// Only first keyword is not surrounded by "".
		{"creation_time: \"" + valid_ts + "\", \"creation_time\": \"" +
			valid_ts + "\"",
			"creation_time: {\"seconds\": " + strconv.Itoa(ts_seconds) +
				", \"nanos\": 0}, \"creation_time\": {\"seconds\": " +
				strconv.Itoa(ts_seconds) + ", \"nanos\": 0}", true},
		// Timestamp is not surrounded by "" and there's other keys and
		// values after.
		{"{\"signed_key\":{\"key\": {\"creation_time\": " + valid_ts +
			", app_id: \"" + primary_test_app_id + "\"}}}",
			"{\"signed_key\":{\"key\": {\"creation_time\": " + valid_ts +
				", app_id: \"" + primary_test_app_id + "\"}}}", true},
	}

	for _, test := range tests {
		r, _ := http.NewRequest("", "", fakeJSONParserReader{bytes.NewBufferString(test.inJSON)})
		err := parseJSON(r, "creation_time")
		if test.outNilErr != (err == nil) {
			t.Errorf("Unexpected err = (%v), want nil = %v", err, test.outNilErr)
		}
		buf := new(bytes.Buffer)
		buf.ReadFrom(r.Body)
		if got, want := buf.String(), test.outJSON; got != want {
			t.Errorf("Out JSON = (%v), want (%v)", got, want)
		}
	}
}
