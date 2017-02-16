package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/urlfetch"
	"log"
	"net/http"
	"os"
	"time"
)

const location = "Asia/Tokyo"

var webhookUrl string = os.Getenv("SLACK_WEBHOOK_URL")

func init() {
	loc, err := time.LoadLocation(location)
	if err != nil {
		loc = time.FixedZone(location, 9*60*60)
	}
	time.Local = loc
	http.HandleFunc("/", Index)
	http.HandleFunc("/tasks/summary/", Summary)
}

func Index(w http.ResponseWriter, r *http.Request) {
	header := r.Header
	en := header.Get("X-GitHub-Event")

	ba := make([]byte, 0, 500)
	p := make([]byte, 100, 100)
	for {
		read_byte, err := r.Body.Read(p)
		if read_byte != 0 {
			ba = append(ba, p[:read_byte]...)
		}
		if err != nil {
			break
		}
	}
	webhook := WebHook{}
	json.Unmarshal(ba, &webhook)

	switch en {
	case "issues":
		issues(webhook, r)
	case "issue_comment":
		issues(webhook, r)
	case "pull_request":
		pullRequest(webhook, r)
	case "pull_request_review_comment":
		pullRequest(webhook, r)
	}
	fmt.Fprint(w, "ok")
}

type WebHook struct {
	Action   string
	Issue    Issue
	Assignee User
	Sender   User
}

type Issue struct {
	Url      string
	Assignee User
}

type User struct {
	Login string
	Id    int64
	Url   string
}

func pullRequest(webhook WebHook, r *http.Request) {
	switch webhook.Action {
	case "opened":
		storeOpenPullRequest(webhook, r)
	case "created":
		storeReviewCommentPullRequest(webhook, r)
	}
}

type PullRequestReviewComment struct {
	Url  string `datastore:",noindex"`
	User string
	Date string
}

func storeReviewCommentPullRequest(webhook WebHook, r *http.Request) {
	now := time.Now()
	ctx := appengine.NewContext(r)
	ic := &PullRequestReviewComment{
		Url:  webhook.Issue.Url,
		User: webhook.Sender.Login,
		Date: now.Format("20060102"),
	}
	key := datastore.NewKey(ctx, "PullRequestReviewComment", "", 0, nil)
	if _, err := datastore.Put(ctx, key, ic); err != nil {
		log.Print(err)
	}
}

type PullRequestOpen struct {
	Url  string `datastore:",noindex"`
	User string
	Date string
}

func storeOpenPullRequest(webhook WebHook, r *http.Request) {
	now := time.Now()
	ctx := appengine.NewContext(r)
	ic := &PullRequestOpen{
		Url:  webhook.Issue.Url,
		User: webhook.Sender.Login,
		Date: now.Format("20060102"),
	}
	key := datastore.NewKey(ctx, "PullRequestOpen", "", 0, nil)
	if _, err := datastore.Put(ctx, key, ic); err != nil {
		log.Print(err)
	}
}

func issues(webhook WebHook, r *http.Request) {
	switch webhook.Action {
	case "assigned":
		//toSlackAssigned(webhook, r)
	case "opened": // Issue Open
		storeOpenIssue(webhook, r)
	case "created": // Issue Comment
		storeCommentIssue(webhook, r)
	}
}

type IssueOpen struct {
	Url  string `datastore:",noindex"`
	User string
	Date string
}

func storeOpenIssue(webhook WebHook, r *http.Request) {
	now := time.Now()
	ctx := appengine.NewContext(r)
	ic := &IssueOpen{
		Url:  webhook.Issue.Url,
		User: webhook.Sender.Login,
		Date: now.Format("20060102"),
	}
	key := datastore.NewKey(ctx, "IssueOpen", "", 0, nil)
	if _, err := datastore.Put(ctx, key, ic); err != nil {
		log.Print(err)
	}
}

type IssueComment struct {
	Url  string `datastore:",noindex"`
	User string
	Date string
}

func storeCommentIssue(webhook WebHook, r *http.Request) {
	now := time.Now()
	ctx := appengine.NewContext(r)
	ic := &IssueOpen{
		Url:  webhook.Issue.Url,
		User: webhook.Sender.Login,
		Date: now.Format("20060102"),
	}
	key := datastore.NewKey(ctx, "IssueComment", "", 0, nil)
	if _, err := datastore.Put(ctx, key, ic); err != nil {
		log.Print(err)
	}
}

func Summary(w http.ResponseWriter, r *http.Request) {
	sm1, err := selectIssueOpen(r)
	if err != nil {
		http.Error(w, err.Error(), 500)
	}
	sm2, err := selectIssueComment(r)
	if err != nil {
		http.Error(w, err.Error(), 500)
	}
	sm3, err := selectPullRequestOpen(r)
	if err != nil {
		http.Error(w, err.Error(), 500)
	}
	sm4, err := selectPullRequestReviewComment(r)
	if err != nil {
		http.Error(w, err.Error(), 500)
	}
	j, err := json.Marshal(&SlackReq{
		Fallback: "Summary Data GitHubAction",
		Fields: []SlackMessage{sm1, sm2, sm3, sm4,
			SlackMessage{
				Title: "今日も1日おつかれさまでした :whale: ",
				Value: "",
				Short: false,
			}},
	})
	if err != nil {
		log.Fatal(err)
	}
	pushSlack(j, r)
}

func selectPullRequestReviewComment(r *http.Request) (SlackMessage, error) {
	now := time.Now()
	ctx := appengine.NewContext(r)
	q := datastore.NewQuery("PullRequestReviewComment").Filter("Date =", now.Format("20060102"))
	var ica []PullRequestReviewComment
	if _, err := q.GetAll(ctx, &ica); err != nil {
		return SlackMessage{}, err
	}
	m := make(map[string]string)
	for i := 0; i < len(ica); i++ {
		ic := ica[i]
		m[ic.User] = m[ic.User] + "■"
	}
	var message string
	for key, value := range m {
		message = message + key + ": " + value + "\n"
	}
	sm := SlackMessage{
		Title: "PullRequest ReviewComment Summary",
		Value: message,
		Short: false,
	}
	return sm, nil
}

func selectPullRequestOpen(r *http.Request) (SlackMessage, error) {
	now := time.Now()
	ctx := appengine.NewContext(r)
	//now.AddDate(0, 0, -1).Format("20060102")
	q := datastore.NewQuery("PullRequestOpen").Filter("Date =", now.Format("20060102"))
	var ica []PullRequestOpen
	if _, err := q.GetAll(ctx, &ica); err != nil {
		return SlackMessage{}, err
	}
	m := make(map[string]string)
	for i := 0; i < len(ica); i++ {
		ic := ica[i]
		m[ic.User] = m[ic.User] + "■"
	}
	var message string
	for key, value := range m {
		message = message + key + ": " + value + "\n"
	}
	sm := SlackMessage{
		Title: "PullRequest Open Summary",
		Value: message,
		Short: false,
	}
	return sm, nil
}

func selectIssueOpen(r *http.Request) (SlackMessage, error) {
	now := time.Now()
	ctx := appengine.NewContext(r)
	q := datastore.NewQuery("IssueOpen").Filter("Date =", now.Format("20060102"))
	var ica []IssueOpen
	if _, err := q.GetAll(ctx, &ica); err != nil {
		return SlackMessage{}, err
	}
	m := make(map[string]string)
	for i := 0; i < len(ica); i++ {
		ic := ica[i]
		m[ic.User] = m[ic.User] + "■"
	}
	var message string
	for key, value := range m {
		message = message + key + ": " + value + "\n"
	}
	sm := SlackMessage{
		Title: "Issue Open Summary",
		Value: message,
		Short: false,
	}
	return sm, nil
}

func selectIssueComment(r *http.Request) (SlackMessage, error) {
	now := time.Now()
	ctx := appengine.NewContext(r)
	q := datastore.NewQuery("IssueComment").Filter("Date =", now.Format("20060102"))
	var ica []IssueComment
	if _, err := q.GetAll(ctx, &ica); err != nil {
		return SlackMessage{}, err
	}
	m := make(map[string]string)
	for i := 0; i < len(ica); i++ {
		ic := ica[i]
		m[ic.User] = m[ic.User] + "■"
	}
	var message string
	for key, value := range m {
		message = message + key + ": " + value + "\n"
	}
	sm := SlackMessage{
		Title: "Issue Comment Summary",
		Value: message,
		Short: false,
	}
	return sm, nil
}

type SlackReq struct {
	Fallback string         `json:"fallback"`
	Pretext  string         `json:"pretext"`
	Color    string         `json:"color"`
	Fields   []SlackMessage `json:"fields"`
}

type SlackMessage struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

func toSlackAssigned(webhook WebHook, r *http.Request) {
	j, err := json.Marshal(&SlackReq{
		Fallback: "Issue Assigne message from GitHub2Slack",
		Color:    "good",
		Fields: []SlackMessage{
			SlackMessage{
				Title: "Issue Assigne message from GitHub2Slack",
				Value: webhook.Assignee.Login + " assigned from " + webhook.Sender.Login + "\n<" + webhook.Issue.Url + "|IssueURL>",
				Short: false,
			},
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	pushSlack(j, r)
}

func pushSlack(j []byte, r *http.Request) {
	req, err := http.NewRequest("POST", webhookUrl, bytes.NewBuffer(j))
	req.Header.Set("Content-Type", "application/json")
	ctx := appengine.NewContext(r)
	client := urlfetch.Client(ctx)
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
}
