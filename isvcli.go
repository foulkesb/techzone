// IBM Technology Zone
// IBM Verify Pattern used to onboard/offboard access to managed aoplications
// Author: Ben Foulkes
// Lastupdated: May 14, 2023
// Usage: go run isvcli.go -ENV=development -GROUP=verify-group-name -API_ID=xxxxxxxxxxxxxxxxxx -API_KEY=xxxxxxxxxxxxxxxxxx -EMAIL=ben.foulkes@ca.ibm.com -TENANT=https://techxchange.verify.ibm.com -ACTION=lookup

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
)

var TechzoneToken string
var FamilyName string
var GivenName string

var authToken string

type jsonField interface{}
type jsonRecord map[string]interface{}
type jsonArray []interface{}

func main() {

	EnvPtr := flag.String("ENV", "development", "production, staging, or development flag")
	APIIDPtr := flag.String("API_ID", "xxxxxxxxxxxxxxx", "API ID")
	APIKeyPtr := flag.String("API_KEY", "xxxxxxxxxxxxxxx", "API Key")
	GroupPtr := flag.String("GROUP", "mygroup-users", "Name of Verify directory group")
	EmailPtr := flag.String("EMAIL", "ben.foulkes@ca.ibm.com", "Email address of user")
	TenantPtr := flag.String("TENANT", "techzone-test.verify.ibm.com/", "Tenant URL")
	ActionPtr := flag.String("ACTION", "lookup", "Verify API Action, lookup, onboard, remove")
	flag.Parse()

	fmt.Println("Group: ", *GroupPtr)

	LOG_FILE := "./output" + *ActionPtr + ".log"
	fmt.Println("Logfile: " + LOG_FILE)
	logFile, err := os.OpenFile(LOG_FILE, os.O_APPEND|os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		log.Panic(err)
	}
	log.SetOutput(logFile)
	log.Println("Running " + *ActionPtr + " for user " + *EmailPtr + " to tenant " + *TenantPtr + " in " + *EnvPtr)

	tokenData := url.Values{}
	tokenData.Set("client_id", *APIIDPtr)
	tokenData.Set("client_secret", *APIKeyPtr)
	tokenData.Set("grant_type", "client_credentials")

	client := http.Client{}
	req, err := http.NewRequest("POST", *TenantPtr+"/v1.0/endpoint/default/token", strings.NewReader(tokenData.Encode()))
	if err != nil {
		//Handle Error
	}

	req.Header = http.Header{
		"Content-Type": []string{"application/x-www-form-urlencoded"},
		"Accept":       []string{"application/json"},
	}

	tokenResp, tokenErr := client.Do(req)
	//Handle Error
	if tokenErr != nil {
		log.Fatalf("An Error Occured %v", tokenErr)
	}
	defer tokenResp.Body.Close()
	//Read the response body
	tokenBody, err := ioutil.ReadAll(tokenResp.Body)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println("Token result:", string(tokenBody)[:])

	var tokenResult map[string]interface{}
	json.Unmarshal([]byte(tokenBody), &tokenResult)

	fmt.Println("Token result:", tokenResult["access_token"])
	authToken = tokenResult["access_token"].(string)

	switch action := *ActionPtr; action {
	case "lookup":
		log.Println("Running user lookup")
		fmt.Println("User onboarded and has group access?", lookup(*TenantPtr, *GroupPtr, *EmailPtr))
	case "onboard":
		log.Println("Running onboard")
		if !lookup(*TenantPtr, *GroupPtr, *EmailPtr) {
			onboarded := onboard(*TenantPtr, *GroupPtr, *EmailPtr)
			if onboarded {
				fmt.Println("USER SUCCESSFULLY ONBOARDED")
				log.Println("USER SUCCESSFULLY ONBOARDED")
			} else {
				fmt.Println("FAIED TO ONBOARD USER")
				log.Println("FAIED TO ONBOARD USER")
			}
		} else {
			// user already has access, so error out
			log.Println("USER ALREADY HAS ACCESS, PROCESSING TERMINATED")
			fmt.Println("USER ALREADY HAS ACCESS, PROCESSING TERMINATED")
			// os.Exit(1)
		}
	case "remove":
		log.Println("Running user remove")
		if offboard(*TenantPtr, *GroupPtr, *EmailPtr) {
			fmt.Println("USER SUCCESSFULLY OFFBOARDED")
			log.Println("USER SUCCESSFULLY OFFBOARDED")
		} else {
			fmt.Println("USER FAILED TO BE OFFBOARDED")
			log.Println("USER FAILED TO BE OFFBOARDED")
			os.Exit(1)
		}
	default:
		log.Println("Action unknown.")
	}
	defer logFile.Close()
}

/*
//////////////////// PUBLIC ROUTING FUNCTIONS FOR CLI ////////////////
// lookup:  returns true if user exists and has access to group
// onboard:  adds user to tenant and grants acccess to group
// offboard:  revokes access from group (but user remains in tenant)
//////////////////////////////////////////////////////////////////////
*/

func lookup(tenant string, group string, email string) bool {

	userId := getUser(tenant, email)
	if userId == "false" {
		fmt.Println("NO SUCH USER")
		return false
	}

	if hasAccess(tenant, group, email) {
		fmt.Println("USER HAS ACCESS")
		return true
	} else {
		fmt.Println("USER DOES NOT HAVE ACCESS")
		return false
	}
}

func onboard(tenant string, group string, email string) bool {

	var userId = "false"
	var groupId = "false"
	groupId = getGroupId(tenant, group)
	userId = getUser(tenant, email)
	if userId == "false" {
		log.Println("onBoard: User does not exist, so adding them")
		fmt.Println("onBoard: User does not exist, so adding them")

		if groupId == "false" {
			log.Println("onBoard: no such group", group)
			fmt.Println("onBoard: no such group", group)
			// no such group id
			return false
		}
		userId = addUser(tenant, email)

		if userId == "false" {
			//failed to add new user
			log.Println("FAILED TO ADD NEW USER")
			os.Exit(1)
		} else {
			fmt.Println("onBoard: User ID:", userId)
			// now add to group
			fmt.Println("onBoard: User just onboarded so now adding to group ", userId, groupId)
			grantAccess(tenant, groupId, userId)
		}
	} else {
		// user exists so just add to group
		fmt.Println("onBoard: User exists so adding to group ", userId, groupId)
		grantAccess(tenant, groupId, userId)
	}

	if userId != "false" {
		log.Println("onBoard: User successfully created")
		return true

	} else {
		log.Println("Failed to create user")
		os.Exit(1)
		return false
	}
}

func offboard(tenant string, group string, email string) bool {
	var userId = "false"
	var groupId = "false"

	groupId = getGroupId(tenant, group)
	userId = getUser(tenant, email)
	if userId == "false" || groupId == "false" {
		log.Println("offBoard: User or group does not exist")
		fmt.Println("offBoard: User or group does not exist")
		return false
	}
	return revokeAccess(tenant, groupId, userId)
}

/*
/////////////////////////// PRIVATE FUNCTIONS ////////////////////////
// hasAccess:  returns true if user has access to group
// getGroupId:  returns ID for a group name, otherwise "false"
// grantAccess:  grants access for a group id to a user id
// getUser:  returns ID for a username otherwise "false"
// addUser:  adds a user to the tenant
// revokeAccess:  removes user from a group
//////////////////////////////////////////////////////////////////////
*/

func hasAccess(tenant string, group string, email string) bool {
	client := http.Client{}

	groupId := getGroupId(tenant, group)
	if groupId == "false" {
		//no such group
		fmt.Println("NO SUCH GROUP")
		return false
	}

	//GROUP ACCESS?
	fmt.Println("Group acces URL:", tenant+"/v2.0/Groups/"+groupId+"?membershipType=firstLevelUsersAndGroups")
	accessReq, _ := http.NewRequest("GET", tenant+"/v2.0/Groups/"+groupId+"?membershipType=firstLevelUsersAndGroups", nil)

	accessReq.Header = http.Header{
		"Authorization": []string{"Bearer " + authToken},
		"Accept":        []string{"application/scim+json"},
	}

	accessResp, err := client.Do(accessReq)
	if err != nil {
		log.Println(err)
	}
	accessBody, err := ioutil.ReadAll(accessResp.Body)
	var accessResult map[string]interface{}
	json.Unmarshal([]byte(accessBody), &accessResult)
	defer accessResp.Body.Close()

	fmt.Println("acccess group result ", accessResult["members"])
	if accessResult["members"] == nil {
		return false
	}

	var foundAccess bool
	foundAccess = false
	for key, element := range accessResult["members"].([]interface{}) {
		accessData := element.(map[string]interface{})
		accessEmails := accessData["emails"].([]interface{})
		fmt.Println("access element:", key, accessEmails[0])
		u := accessEmails[0].(map[string]interface{})
		fmt.Println("access element1:", u["value"])
		if strings.ToLower(u["value"].(string)) == strings.ToLower(email) {
			fmt.Println("Found user in group!", u["value"])
			foundAccess = true
		}

	}

	fmt.Println("hasAccess: Found access? ", foundAccess)
	log.Println("hasAccess: Found access? ", foundAccess)

	return foundAccess
}

func getGroupId(tenant string, group string) string {
	client := http.Client{}

	groupsReq, _ := http.NewRequest("GET", tenant+"/v2.0/Groups", nil)

	groupsReq.Header = http.Header{
		"Authorization": []string{"Bearer " + authToken},
		"Accept":        []string{"application/scim+json"},
	}

	groupsResp, err := client.Do(groupsReq)
	if err != nil {
		log.Println(err)
	}
	groupsBody, err := ioutil.ReadAll(groupsResp.Body)
	var groupsResult map[string]interface{}
	json.Unmarshal([]byte(groupsBody), &groupsResult)

	groupList := groupsResult["Resources"].([]interface{})

	defer groupsResp.Body.Close()

	var groupId string

	for _, element := range groupList {
		groupData := element.(map[string]interface{})
		fmt.Println(groupData["id"], groupData["displayName"])
		if strings.ToLower(groupData["displayName"].(string)) == strings.ToLower(group) {
			groupId = groupData["id"].(string)
			fmt.Println("getGroupId: Found group!", group, groupId)
			fmt.Println("getGroupId: Found group? ", groupId)
			log.Println("getGroupId: Found group? ", groupId)
			return groupId
		}
	}

	return "false"
}

func grantAccess(tenant string, groupId string, userId string) bool {
	client := http.Client{}

	groupPayload := `{ "schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
	"Operations": [ 
		{ "op": "add", "path": "members", "value": [ 
			{ "type": "user", "value": "` + userId + `" } ] }
	]}`

	var requestPayload map[string]interface{}
	json.Unmarshal([]byte(groupPayload), &requestPayload)
	groupJSON, _ := json.Marshal(requestPayload)
	fmt.Println("group access payload json", bytes.NewBuffer(groupJSON))

	accessReq, _ := http.NewRequest("PATCH", tenant+"/v2.0/Groups/"+groupId, bytes.NewBuffer(groupJSON))

	accessReq.Header = http.Header{
		"Authorization": []string{"Bearer " + authToken},
		"Content-Type":  []string{"application/scim+json"},
		"Accept":        []string{"application/scim+json"},
	}

	accessResp, err := client.Do(accessReq)
	if err != nil {
		log.Println(err)
	}
	accessBody, err := ioutil.ReadAll(accessResp.Body)
	var accessResult map[string]interface{}
	json.Unmarshal([]byte(accessBody), &accessResult)
	defer accessResp.Body.Close()

	fmt.Println("acccess group result ", accessResult)
	if accessResult == nil && accessResp.StatusCode != 201 {
		//failed to grant access
		return false
	}

	return true
}

func getUser(tenant string, email string) string {

	client := http.Client{}

	req, _ := http.NewRequest("GET", tenant+"/v2.0/Users?filter=emails+eq+%22"+email+"%22", nil)

	req.Header = http.Header{
		"Authorization": []string{"Bearer " + authToken},
		"Accept":        []string{"application/scim+json"},
	}

	userResp, err := client.Do(req)
	if err != nil {
		log.Println(err)
	}

	log.Println()

	userBody, err := ioutil.ReadAll(userResp.Body)

	var userResult map[string]interface{}
	json.Unmarshal([]byte(userBody), &userResult)
	fmt.Println("getUser:", userResp.StatusCode, userResult)
	log.Println("getUser:", userResp.StatusCode, userResult)
	defer userResp.Body.Close()

	if userResp.StatusCode != 200 {
		return "false"
	}

	userData := userResult["Resources"].([]interface{})

	if len(userData) == 0 {
		//no data
		return "false"
	}

	s := userData[0].(map[string]interface{})

	userId := s["id"].(string)
	fmt.Println("getUser: User ID:", userId)

	return userId
}

func addUser(tenant string, email string) string {

	client := http.Client{}

	userPayload := `{  "schemas": [
		  "urn:ietf:params:scim:schemas:core:2.0:User",
		  "urn:ietf:params:scim:schemas:extension:ibm:2.0:User",
		  "urn:ietf:params:scim:schemas:extension:enterprise:2.0:User",
		  "urn:ietf:params:scim:schemas:extension:ibm:2.0:Notification"
	  ], 
	"userName": "` + email + `@www.ibm.com",
	"name": { 
		"familyName": "` + FamilyName + `", 
		"givenName": "` + GivenName + `" }, 
		"emails":[ {"type":"work", "value": "` + email + `"} ],
		"urn:ietf:params:scim:schemas:extension:ibm:2.0:User": {
			"realm": "www.ibm.com",
			"userCategory": "federated", "twoFactorAuthentication": false },
			"urn:ietf:params:scim:schemas:extension:enterprise:2.0:User": {
    		"department": "Techzone Users" } }`

	fmt.Println("user payload string", userPayload)

	var requestPayload map[string]interface{}
	json.Unmarshal([]byte(userPayload), &requestPayload)
	userJSON, _ := json.Marshal(requestPayload)
	log.Println("addUser: userBody", json.Valid(userJSON), bytes.NewBuffer(userJSON))
	fmt.Println("addUser: userBody", json.Valid(userJSON), bytes.NewBuffer(userJSON))

	req, _ := http.NewRequest("POST", tenant+"/v2.0/Users", bytes.NewBuffer(userJSON))

	req.Header = http.Header{
		"Authorization": []string{"Bearer " + authToken},
		"Content-Type":  []string{"application/scim+json"},
		"Accept":        []string{"application/scim+json"},
	}

	userResp, err := client.Do(req)
	if err != nil {
		log.Println(err)
	}

	log.Println()

	userBody, err := ioutil.ReadAll(userResp.Body)
	fmt.Println("addUser: User result:", string(userBody)[:])
	log.Println("addUser: User result:", string(userBody)[:])

	var userResult map[string]interface{}
	json.Unmarshal([]byte(userBody), &userResult)
	fmt.Println("addUser: ", userResp.StatusCode, userResult)
	log.Println("addUser: ", userResp.StatusCode, userResult)
	defer userResp.Body.Close()

	if userResp.StatusCode >= 400 {
		return "false"
	}

	userId := userResult["id"].(string)
	fmt.Println("User ID:", userId)

	return userId
}

func revokeAccess(tenant string, groupId string, userId string) bool {
	client := http.Client{}

	//GRANT GROUP ACCESS

	groupPayload := `{ "schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
	"Operations": [ 
		{ "op": "remove", "path": "members[value eq \"` + userId + `\"]" } ] }`

	fmt.Println("revoke payload string: ", groupPayload)
	var requestPayload map[string]interface{}
	json.Unmarshal([]byte(groupPayload), &requestPayload)
	groupJSON, _ := json.Marshal(requestPayload)
	fmt.Println("group revoke payload json", bytes.NewBuffer(groupJSON))

	accessReq, _ := http.NewRequest("PATCH", tenant+"/v2.0/Groups/"+groupId, bytes.NewBuffer(groupJSON))

	accessReq.Header = http.Header{
		"Authorization": []string{"Bearer " + authToken},
		"Content-Type":  []string{"application/scim+json"},
		"Accept":        []string{"application/scim+json"},
	}

	accessResp, err := client.Do(accessReq)
	if err != nil {
		log.Println(err)
	}
	accessBody, err := ioutil.ReadAll(accessResp.Body)
	var accessResult map[string]interface{}
	json.Unmarshal([]byte(accessBody), &accessResult)
	defer accessResp.Body.Close()

	fmt.Println("revokeAccess: Acccess group result ", accessResp.StatusCode, accessResult)
	if accessResult == nil && accessResp.StatusCode >= 400 {
		//failed to revoke access
		return false
	}

	return true
}
