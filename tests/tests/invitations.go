package tests

import (
	"bytes"
	stdcontext "context"
	"encoding/json"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"tests/util"
)

func invitationCSV(c *util.Context, content, headerField string, noSend bool) (int, string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	file, err := writer.CreateFormFile("csv", "judges.csv")
	if err != nil {
		return 0, "", err
	}
	if _, err = io.WriteString(file, content); err != nil {
		return 0, "", err
	}
	if headerField != "" {
		writer.WriteField(headerField, "true")
	}
	writer.WriteField("noSend", fmt.Sprint(noSend))
	writer.Close()
	req, err := http.NewRequest("POST", os.Getenv("API_URL")+"/judge/csv", &body)
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Authorization", util.AdminAuth())
	req.Header.Set("Content-Type", writer.FormDataContentType())
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	return response.StatusCode, string(data), err
}

// HTTP checks never send external mail. Failure/retry is exercised with invalid
// recipient fixtures; successful transport and partial failure use injected fake
// senders in server/funcs/invitations_test.go.
func JudgeCSVInvitations(c *util.Context) util.Result {
	defer resetGroupOptions(c)
	if res := setupTrackGroups(c); !res.Success {
		return res
	}
	ctx := stdcontext.Background()
	for _, path := range []string{"/admin/invitations", "/admin/invitations/retry", "/judge/" + primitive.NewObjectID().Hex() + "/invitation"} {
		status, _ := util.PostRequestWithStatus(c.Logger, path, nil, util.DefaultAuth())
		if path == "/admin/invitations" {
			status, _ = util.GetRequestWithStatus(c.Logger, path, util.DefaultAuth())
		}
		if status < 400 {
			return util.NewResult(false, "invitation endpoint exposed without admin auth")
		}
	}
	input := "\ufeffName,Email,Track,Notes\n A , shared@example.com ,Track A,\"notes, with commas\"\nG,shared@example.com,,\n"
	status, body, err := invitationCSV(c, input, "hasHeader", true)
	if err != nil || status != 200 {
		return util.NewResult(false, fmt.Sprintf("header import: %d %s %v", status, body, err))
	}
	var judges []struct {
		ID     string `json:"id"`
		Code   string `json:"code"`
		Name   string `json:"name"`
		Email  string `json:"email"`
		Track  string `json:"track"`
		Notes  string `json:"notes"`
		Status string `json:"invitation_status"`
	}
	if err := jsonUnmarshalList(util.GetRequest(c.Logger, "/judge/list", util.AdminAuth()), &judges); err != nil {
		return util.NewResult(false, err.Error())
	}
	if len(judges) != 2 {
		return util.NewResult(false, "header imported as judge or rows missing")
	}
	for _, j := range judges {
		if j.Status != "not_sent" || j.Email != "shared@example.com" || len(j.Code) != 8 {
			return util.NewResult(false, "invalid imported account")
		}
	}
	status, _, err = invitationCSV(c, input, "hasHeader", true)
	if err != nil || status != 409 {
		return util.NewResult(false, "repeat import was not rejected")
	}
	for _, bad := range []string{
		"Missing,,Track A,\nValid,valid@example.com,Track A,\n",
		"Typo,typo@example.com,Unknown Track,\n",
		"Dup,dup@example.com,,\nDup2,DUP@example.com,,\n",
		"\n",
	} {
		status, _, err = invitationCSV(c, bad, "", false)
		if err != nil || status != 400 {
			return util.NewResult(false, "invalid batch was accepted")
		}
	}
	count, _ := c.Db.Collection("judges").CountDocuments(ctx, bson.M{})
	if count != 2 {
		return util.NewResult(false, "invalid/repeated import partially created accounts")
	}
	// Accept the previous form field during deployments with a cached frontend.
	status, body, err = invitationCSV(c, "Name,Email\nLegacy,legacy@example.com\n", "headerRow", true)
	if err != nil || status != 200 {
		return util.NewResult(false, "legacy header field not supported: "+body)
	}
	response := util.PostRequest(c.Logger, "/admin/invitations/retry", nil, util.AdminAuth())
	if !strings.Contains(response, "\"accepted\":0") || !strings.Contains(response, "\"failed\":0") {
		return util.NewResult(false, "bulk retry included not-sent accounts")
	}
	// Simulate pending addresses becoming invalid; no email transport is invoked.
	for _, j := range judges {
		id, _ := primitive.ObjectIDFromHex(j.ID)
		if _, err := c.Db.Collection("judges").UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"email": "invalid@", "invitation_status": "pending"}}); err != nil {
			return util.NewResult(false, err.Error())
		}
	}
	response = util.PostRequest(c.Logger, "/admin/invitations/retry", nil, util.AdminAuth())
	if !strings.Contains(response, "\"failed\":2") {
		return util.NewResult(false, "retry did not continue after first failure: "+response)
	}
	summary := util.GetRequest(c.Logger, "/admin/invitations", util.AdminAuth())
	if !strings.Contains(summary, "\"failed\":2") || !strings.Contains(summary, "\"retryable\":2") || strings.Contains(summary, "\"code\"") {
		return util.NewResult(false, "persisted summary incorrect: "+summary)
	}
	// Per-account retries keep codes and do not recreate accounts.
	response = util.PostRequest(c.Logger, "/judge/"+judges[0].ID+"/invitation", nil, util.AdminAuth())
	if !strings.Contains(response, "\"failed\":1") {
		return util.NewResult(false, "per-judge retry failed: "+response)
	}
	id, _ := primitive.ObjectIDFromHex(judges[0].ID)
	var saved struct {
		Code   string `bson:"code"`
		Status string `bson:"invitation_status"`
	}
	if err := c.Db.Collection("judges").FindOne(ctx, bson.M{"_id": id}).Decode(&saved); err != nil {
		return util.NewResult(false, err.Error())
	}
	if saved.Code != judges[0].Code || saved.Status != "failed" {
		return util.NewResult(false, "retry changed account code")
	}
	response = util.PutRequest(c.Logger, "/judge/"+judges[0].ID, util.H{"name": "Corrected", "email": "corrected@example.com", "notes": ""}, util.AdminAuth())
	if !util.IsOk(response) {
		return util.NewResult(false, "could not correct email: "+response)
	}
	c.Db.Collection("judges").FindOne(ctx, bson.M{"_id": id}).Decode(&saved)
	if saved.Status != "not_sent" || saved.Code != judges[0].Code {
		return util.NewResult(false, "email edit did not reset invitation status safely")
	}
	c.Db.Collection("judges").UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"token": "invitation-test-token"}})
	for _, path := range []string{"/admin/invitations/retry", "/judge/" + judges[0].ID + "/invitation"} {
		status, _ := util.PostRequestWithStatus(c.Logger, path, nil, util.JudgeAuth("invitation-test-token"))
		if status < 400 {
			return util.NewResult(false, "judge could send admin invitations")
		}
	}
	status, _ = util.GetRequestWithStatus(c.Logger, "/admin/invitations", util.JudgeAuth("invitation-test-token"))
	if status < 400 {
		return util.NewResult(false, "judge could see invitation roster")
	}
	count, _ = c.Db.Collection("judges").CountDocuments(ctx, bson.M{})
	if count != 3 {
		return util.NewResult(false, "retry created duplicate accounts")
	}
	var info struct {
		Counts map[string]int `json:"counts"`
	}
	if err := json.Unmarshal([]byte(util.GetRequest(c.Logger, "/admin/invitations", util.AdminAuth())), &info); err != nil {
		return util.NewResult(false, err.Error())
	}
	if info.Counts["failed"] != 1 || info.Counts["not_sent"] != 2 {
		return util.NewResult(false, "summary lost stored status")
	}
	return util.ResultOk()
}
