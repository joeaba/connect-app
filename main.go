package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
	"github.com/joho/godotenv"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

// These structs define the data models
type Teams struct {
	Teams map[string]Team `json:"teams"`
}

type Team struct {
	Members []Member `json:"members"`
}

type Member struct {
	MemberID string            `json:"member_id"`
	Name     string            `json:"name"`
	Channels map[string]string `json:"channels"`
}

type Users map[string]User

type User struct {
	MemberID  string            `json:"member_id"`
	Name      string            `json:"name"`
	UpdatedAt time.Time         `json:"updatedAt"`
	Channels  map[string]string `json:"channels"`
}

type Channels map[string]Channel

type Channel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// These constants define the "database" file names
// We use JSON files as a simple data store
const (
	TeamsFile    = "teams.json"
	UsersFile    = "users.json"
	ChannelsFile = "channels.json"
)

var (
	api       *slack.Client
	botUserID string
)

func main() {
	log.Println("Starting Slack Connect Manager...")

	// Load the environment variables
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file: ", err)
	}

	// Initialize the Slack API client
	api = slack.New(os.Getenv("SLACK_BOT_TOKEN"))
	log.Println("Slack API client initialized")

	// Get the bot's user ID
	authTest, err := api.AuthTest()
	if err != nil {
		log.Fatalf("Error getting bot user ID: %v", err)
	}
	botUserID = authTest.UserID
	log.Printf("Bot User ID: %s", botUserID)

	// Make sure the data files exist. If they don't, it creates them
	ensureFileExists(TeamsFile)
	ensureFileExists(UsersFile)
	ensureFileExists(ChannelsFile)

	// Set up our HTTP handlers
	http.HandleFunc("/slack/events", handleSlackEvent)
	http.HandleFunc("/slack/command", handleSlackCommand)

	// Start the user info update routine in the background
	go updateUserInfo()

	// Start the server
	log.Println("Server listening on :3000")
	log.Fatal(http.ListenAndServe(":3000", nil))
}

// Function to make sure our data files exist.
func ensureFileExists(filename string) {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		log.Printf("Creating %s file", filename)
		file, err := os.Create(filename)
		if err != nil {
			log.Fatalf("Error creating %s: %v", filename, err)
		}
		file.Close()
		
		// Initialize with empty data
		switch filename {
		case TeamsFile:
			writeTeams(Teams{Teams: make(map[string]Team)})
		case UsersFile:
			writeUsers(make(Users))
		case ChannelsFile:
			writeChannels(make(Channels))
		}
	}
}

// Handle Slack events. Right now, it just handles URL verification
func handleSlackEvent(w http.ResponseWriter, r *http.Request) {
	log.Println("Received Slack event")

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	ev, err := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionNoVerifyToken())
	if err != nil {
		log.Printf("Error parsing Slack event: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if ev.Type == slackevents.URLVerification {
		var r *slackevents.ChallengeResponse
		err := json.Unmarshal([]byte(body), &r)
		if err != nil {
			log.Printf("Error unmarshalling challenge response: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text")
		w.Write([]byte(r.Challenge))
		log.Println("Responded to URL verification challenge")
	}
}

// Handle all the Slack commands
func handleSlackCommand(w http.ResponseWriter, r *http.Request) {
	log.Println("Received a request to /slack/command")
	log.Printf("Headers: %+v", r.Header)
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	log.Printf("Body: %s", string(body))
	r.Body = ioutil.NopCloser(bytes.NewBuffer(body))

	s, err := slack.SlashCommandParse(r)
	if err != nil {
		log.Printf("Error parsing slash command: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// We only care about /connect commands
	if s.Command != "/connect" {
		log.Printf("Received unknown command: %s", s.Command)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	log.Printf("Received /connect command with text: %s", s.Text)

	args := strings.Fields(s.Text)
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" {
		showHelp(w)
		return
	}

	action := args[0]
	log.Printf("Processing action: %s", action)

	// Route the command to the appropriate handler
	switch action {
	case "create-team":
		handleCreateTeam(w, args[1:])
	case "remove-team":
		handleRemoveTeam(w, args[1:])
	case "add":
		handleAdd(w, args[1:])
	case "remove":
		handleRemove(w, args[1:])
	case "print":
		handlePrint(w, args[1:])
	case "invite":
		handleInvite(w, args[1:])
	case "ping":
		handlePing(w, args[1:])
	case "add-channel":
		handleAddChannel(w, args[1:], s.ChannelID, s.ChannelName)
	case "remove-channel":
		handleRemoveChannel(w, args[1:])
	default:
		log.Printf("Invalid action received: %s", action)
		showHelp(w)
	}
}

// Show the help message
func showHelp(w http.ResponseWriter) {
	helpText := `Available commands:
- /connect create-team <team>
- /connect remove-team <team>
- /connect add <team> <member_id>
- /connect remove <team> <member_id>
- /connect print teams
- /connect print channels
- /connect print members <team>
- /connect invite <team>
- /connect ping <team> <channel>
- /connect add-channel
- /connect remove-channel <channel>
- /connect help or /connect -h (shows this help message)`

	responseSuccess(w, helpText)
}

// Create a new team
func handleCreateTeam(w http.ResponseWriter, args []string) {
	if len(args) < 1 {
		responseError(w, "Please provide a team name to create.")
		return
	}

	team := args[0]
	teams, err := readTeams()
	if err != nil {
		responseError(w, "Error reading teams.")
		return
	}

	if _, exists := teams.Teams[team]; exists {
		responseError(w, fmt.Sprintf("Team '%s' already exists.", team))
		return
	}

	teams.Teams[team] = Team{Members: []Member{}}
	err = writeTeams(teams)
	if err != nil {
		responseError(w, "Error writing to teams.")
		return
	}

	responseSuccess(w, fmt.Sprintf("Team '%s' has been created.", team))
}

// Remove a team
func handleRemoveTeam(w http.ResponseWriter, args []string) {
	if len(args) < 1 {
		responseError(w, "Please provide a team name to remove.")
		return
	}

	team := args[0]
	teams, err := readTeams()
	if err != nil {
		responseError(w, "Error reading teams.")
		return
	}

	if _, exists := teams.Teams[team]; !exists {
		responseError(w, fmt.Sprintf("Team '%s' does not exist.", team))
		return
	}

	delete(teams.Teams, team)
	err = writeTeams(teams)
	if err != nil {
		responseError(w, "Error writing to teams.")
		return
	}

	responseSuccess(w, fmt.Sprintf("Team '%s' has been removed.", team))
}

// Add a member to a team
func handleAdd(w http.ResponseWriter, args []string) {
	if len(args) < 2 {
		responseError(w, "Please provide a team name and a member ID to add.")
		return
	}

	team, memberID := args[0], args[1]
	teams, err := readTeams()
	if err != nil {
		responseError(w, "Error reading teams.")
		return
	}

	if _, exists := teams.Teams[team]; !exists {
		responseError(w, fmt.Sprintf("Team '%s' does not exist.", team))
		return
	}

	for _, member := range teams.Teams[team].Members {
		if member.MemberID == memberID {
			responseError(w, fmt.Sprintf("User %s is already in team '%s'.", memberID, team))
			return
		}
	}

	// Get info about the user from Slack
	userInfo, err := api.GetUserInfo(memberID)
	if err != nil {
		log.Printf("Error getting user info for %s: %v", memberID, err)
		responseError(w, fmt.Sprintf("Error getting user info: %v", err))
		return
	}

	displayName := userInfo.Profile.DisplayName
	if displayName == "" {
		displayName = userInfo.Name
	}

	log.Printf("Adding user %s with display name %s to team %s", memberID, displayName, team)

	newMember := Member{
		MemberID: memberID,
		Name:     displayName,
		Channels: make(map[string]string),
	}
	updatedTeam := teams.Teams[team]
	updatedTeam.Members = append(updatedTeam.Members, newMember)
	teams.Teams[team] = updatedTeam

	err = writeTeams(teams)
	if err != nil {
		responseError(w, "Error writing to teams.")
		return
	}

	// Add this member to the users file
	users, err := readUsers()
	if err != nil {
		log.Printf("Error reading users: %v", err)
	} else {
		users[memberID] = User{
			MemberID:  memberID,
			Name:      displayName,
			UpdatedAt: time.Now(),
			Channels:  make(map[string]string),
		}
		err = writeUsers(users)
		if err != nil {
			log.Printf("Error writing users: %v", err)
		}
	}

	responseSuccess(w, fmt.Sprintf("Added user %s (%s) to team '%s'.", displayName, memberID, team))
}

// Remove a member from a team
func handleRemove(w http.ResponseWriter, args []string) {
	if len(args) < 2 {
		responseError(w, "Please provide a team name and a member ID to remove.")
		return
	}

	team, memberID := args[0], args[1]
	teams, err := readTeams()
	if err != nil {
		responseError(w, "Error reading teams.")
		return
	}

	if _, exists := teams.Teams[team]; !exists {
		responseError(w, fmt.Sprintf("Team '%s' does not exist.", team))
		return
	}

	found := false
	updatedTeam := teams.Teams[team]
	for i, member := range updatedTeam.Members {
		if member.MemberID == memberID {
			updatedTeam.Members = append(updatedTeam.Members[:i], updatedTeam.Members[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		responseError(w, fmt.Sprintf("User %s is not in team '%s'.", memberID, team))
		return
	}

	teams.Teams[team] = updatedTeam
	err = writeTeams(teams)
	if err != nil {
		responseError(w, "Error writing to teams.")
		return
	}

	responseSuccess(w, fmt.Sprintf("Removed user %s from team '%s'.", memberID, team))
}

// Print information about teams, channels, or members
func handlePrint(w http.ResponseWriter, args []string) {
	if len(args) < 1 {
		responseError(w, "Please specify what to print: teams, channels, or members <team>.")
		return
	}

	option := args[0]
	switch option {
	case "teams":
		printTeams(w)
	case "channels":
		printChannels(w)
	case "members":
		if len(args) < 2 {
			responseError(w, "Please provide a team name to print members.")
			return
		}
		printMembers(w, args[1])
	default:
		responseError(w, "Invalid print option. Use 'teams', 'channels', or 'members <team>'.")
	}
}

// Print all teams
func printTeams(w http.ResponseWriter) {
	teams, err := readTeams()
	if err != nil {
		responseError(w, "Error reading teams.")
		return
	}

	teamNames := make([]string, 0, len(teams.Teams))
	for team := range teams.Teams {
		teamNames = append(teamNames, team)
	}

	if len(teamNames) == 0 {
		responseSuccess(w, "No teams found.")
	} else {
		responseSuccess(w, fmt.Sprintf("Teams: %s", strings.Join(teamNames, ", ")))
	}
}

// Print all channels
func printChannels(w http.ResponseWriter) {
	channels, err := readChannels()
	if err != nil {
		responseError(w, "Error reading channels.")
		return
	}

	channelNames := make([]string, 0, len(channels))
	for _, channel := range channels {
		channelNames = append(channelNames, channel.Name)
	}

	if len(channelNames) == 0 {
		responseSuccess(w, "No channels found.")
	} else {
		responseSuccess(w, fmt.Sprintf("Channels: %s", strings.Join(channelNames, ", ")))
	}
}

// Print all members of a specific team
func printMembers(w http.ResponseWriter, team string) {
	teams, err := readTeams()
	if err != nil {
		responseError(w, "Error reading teams.")
		return
	}

	if _, exists := teams.Teams[team]; !exists {
		responseError(w, fmt.Sprintf("Team '%s' does not exist.", team))
		return
	}

	members := make([]string, len(teams.Teams[team].Members))
	for i, member := range teams.Teams[team].Members {
		if member.Name != "" {
			members[i] = fmt.Sprintf("%s (%s)", member.Name, member.MemberID)
		} else {
			members[i] = member.MemberID
		}
	}

	if len(members) == 0 {
		responseSuccess(w, fmt.Sprintf("No members found in team '%s'.", team))
	} else {
		responseSuccess(w, fmt.Sprintf("Members of team '%s': %s", team, strings.Join(members, ", ")))
	}
}

// Handle the invite command
func handleInvite(w http.ResponseWriter, args []string) {
	if len(args) < 1 {
		responseError(w, "Please provide a team name for invitation.")
		return
	}

	team := args[0]
	teams, err := readTeams()
	if err != nil {
		responseError(w, "Error reading teams.")
		return
	}

	if _, exists := teams.Teams[team]; !exists {
		responseError(w, fmt.Sprintf("Team '%s' does not exist.", team))
		return
	}

	memberIDs := make([]string, len(teams.Teams[team].Members))
	for i, member := range teams.Teams[team].Members {
		memberIDs[i] = member.MemberID
	}

	responseSuccess(w, fmt.Sprintf("To invite team '%s', use these member IDs: %s", team, strings.Join(memberIDs, ", ")))
}

// Ping all members of a team in a specific channel
func handlePing(w http.ResponseWriter, args []string) {
	if len(args) < 2 {
		responseError(w, "Please provide a team name and a channel name to ping.")
		return
	}

	team, channelName := args[0], args[1]
	log.Printf("Attempting to ping team '%s' in channel '%s'", team, channelName)

	teams, err := readTeams()
	if err != nil {
		log.Printf("Error reading teams: %v", err)
		responseError(w, "Error reading teams.")
		return
	}

	if _, exists := teams.Teams[team]; !exists {
		log.Printf("Team '%s' does not exist", team)
		responseError(w, fmt.Sprintf("Team '%s' does not exist.", team))
		return
	}

	channels, err := readChannels()
	if err != nil {
		log.Printf("Error reading channels: %v", err)
		responseError(w, "Error reading channels.")
		return
	}

	var channelID string
	for id, channel := range channels {
		if channel.Name == channelName {
			channelID = id
			break
		}
	}

	if channelID == "" {
		log.Printf("Channel '%s' not found", channelName)
		responseError(w, fmt.Sprintf("Channel '%s' not found.", channelName))
		return
	}

	log.Printf("Found channel ID '%s' for channel name '%s'", channelID, channelName)

	var mentions []string
	for _, member := range teams.Teams[team].Members {
		if member.MemberID != "" {
			log.Printf("Adding member '%s' to mentions", member.MemberID)
			mentions = append(mentions, fmt.Sprintf("<@%s>", member.MemberID))
		}
	}

	if len(mentions) == 0 {
		log.Printf("No members found in team '%s'", team)
		responseError(w, fmt.Sprintf("No members of team '%s' found.", team))
		return
	}

	log.Printf("Attempting to post message to channel '%s' with mentions: %v", channelID, mentions)
	_, _, err = api.PostMessage(channelID, slack.MsgOptionText(strings.Join(mentions, " "), false))
	if err != nil {
		log.Printf("Error pinging team: %v", err)
		responseError(w, fmt.Sprintf("Error pinging team: %v", err))
		return
	}

	log.Printf("Successfully pinged team '%s' in channel '%s'", team, channelName)
	responseSuccess(w, fmt.Sprintf("Successfully pinged team '%s' in #%s.", team, channelName))
}

// Add a channel to the tracking list
func handleAddChannel(w http.ResponseWriter, args []string, channelID, channelName string) {
	log.Printf("Attempting to add channel %s (%s)", channelName, channelID)

	// Check if we're actually in the channel we're trying to add
	if channelID == "" || channelName == "" {
		log.Printf("Error: Channel ID or name is empty")
		responseError(w, "You need to run the command inside the channel you want to add. If you are trying to add a private channel please run /invite @connect-management.")
		return
	}

	channels, err := readChannels()
	if err != nil {
		log.Printf("Error reading channels: %v", err)
		responseError(w, "You need to run the command inside the channel you want to add. If you are trying to add a private channel please run /invite @connect-management.")
		return
	}

	if _, exists := channels[channelID]; exists {
		log.Printf("Channel #%s is already being tracked.", channelName)
		responseError(w, fmt.Sprintf("Channel #%s is already being tracked.", channelName))
		return
	}

	// Try to join the channel
	_, _, _, err = api.JoinConversation(channelID)
	if err != nil {
		log.Printf("Error joining channel: %v", err)
		responseError(w, "You need to run the command inside the channel you want to add. If you are trying to add a private channel please run /invite @connect-management.")
		return
	}

	channels[channelID] = Channel{
		ID:   channelID,
		Name: channelName,
	}
	err = writeChannels(channels)
	if err != nil {
		log.Printf("Error writing to channels file: %v", err)
		responseError(w, "Error writing to channels file.")
		return
	}

	// Update user information for this channel
	go updateUserInfoForChannel(channelID)

	log.Printf("Successfully added channel #%s to the tracking list.", channelName)
	responseSuccess(w, fmt.Sprintf("Channel #%s has been added to the tracking list.", channelName))
}

// Remove a channel from the tracking list
func handleRemoveChannel(w http.ResponseWriter, args []string) {
	if len(args) < 1 {
		responseError(w, "Please provide a channel name to remove.")
		return
	}

	channelName := args[0]
	channels, err := readChannels()
	if err != nil {
		responseError(w, "Error reading channels.")
		return
	}

	var channelID string
	for id, channel := range channels {
		if channel.Name == channelName {
			channelID = id
			break
		}
	}

	if channelID == "" {
		responseError(w, fmt.Sprintf("Channel #%s is not being tracked.", channelName))
		return
	}

	delete(channels, channelID)
	err = writeChannels(channels)
	if err != nil {
		responseError(w, "Error writing to channels file.")
		return
	}

	responseSuccess(w, fmt.Sprintf("Channel #%s has been removed from the tracking list.", channelName))
}

// Update the user info
func updateUserInfo() {
	log.Println("Starting user info update routine")
	for {
		log.Println("Updating user info")
		channels, err := readChannels()
		if err != nil {
			log.Printf("Error reading channels: %v", err)
			time.Sleep(10 * time.Second)
			continue
		}

		for channelID := range channels {
			updateUserInfoForChannel(channelID)
		}

		log.Println("User info update completed")
		time.Sleep(10 * time.Second)
	}
}

// Update user info for a specific channel
func updateUserInfoForChannel(channelID string) {
	log.Printf("Updating users for channel %s", channelID)

	users, err := readUsers()
	if err != nil {
		log.Printf("Error reading users: %v", err)
		return
	}

	teams, err := readTeams()
	if err != nil {
		log.Printf("Error reading teams: %v", err)
		return
	}

	members, _, err := api.GetUsersInConversation(&slack.GetUsersInConversationParameters{
		ChannelID: channelID,
	})
	if err != nil {
		log.Printf("Error getting users in channel %s: %v", channelID, err)
		return
	}

	log.Printf("Found %d members in channel %s", len(members), channelID)

	for _, memberID := range members {
		userInfo, err := api.GetUserInfo(memberID)
		if err != nil {
			log.Printf("Error getting user info for %s: %v", memberID, err)
			continue
		}

		if userInfo.IsBot {
			log.Printf("Skipping bot user %s", memberID)
			continue
		}

		displayName := userInfo.Profile.DisplayName
		if displayName == "" {
			displayName = userInfo.Name
		}

		log.Printf("Updating info for user %s (%s)", memberID, displayName)

		user, exists := users[memberID]
		if !exists {
			user = User{
				MemberID: memberID,
				Name:     displayName,
				Channels: make(map[string]string),
			}
		} else {
			user.Name = displayName
		}
		user.UpdatedAt = time.Now()
		user.Channels[channelID] = memberID
		users[memberID] = user

		// Update team members
		for teamName, team := range teams.Teams {
			for i, member := range team.Members {
				if member.MemberID == memberID {
					updatedMember := member
					updatedMember.Name = displayName
					if updatedMember.Channels == nil {
						updatedMember.Channels = make(map[string]string)
					}
					updatedMember.Channels[channelID] = memberID
					teams.Teams[teamName].Members[i] = updatedMember
					log.Printf("Updated member %s in team %s", memberID, teamName)
				}
			}
		}
	}

	err = writeUsers(users)
	if err != nil {
		log.Printf("Error writing users: %v", err)
	}

	err = writeTeams(teams)
	if err != nil {
		log.Printf("Error writing teams: %v", err)
	}

	log.Printf("Finished updating users for channel %s", channelID)
}

// Read teams from the JSON file
func readTeams() (Teams, error) {
	log.Println("Reading teams")
	var teams Teams
	data, err := ioutil.ReadFile(TeamsFile)
	if err != nil {
		if os.IsNotExist(err) {
			log.Println("Teams file does not exist, creating new")
			return Teams{Teams: make(map[string]Team)}, nil
		}
		return teams, err
	}
	err = json.Unmarshal(data, &teams)
	return teams, err
}

// Write teams to the JSON file
func writeTeams(teams Teams) error {
	log.Println("Writing teams")
	data, err := json.MarshalIndent(teams, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(TeamsFile, data, 0644)
}

// Read users from the JSON file
func readUsers() (Users, error) {
	log.Println("Reading users")
	var users Users
	data, err := ioutil.ReadFile(UsersFile)
	if err != nil {
		if os.IsNotExist(err) {
			log.Println("Users file does not exist, creating new")
			return make(Users), nil
		}
		return users, err
	}
	err = json.Unmarshal(data, &users)
	return users, err
}

// Write users to the JSON file
func writeUsers(users Users) error {
	log.Println("Writing users")
	data, err := json.MarshalIndent(users, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(UsersFile, data, 0644)
}

// Read channels from the JSON file
func readChannels() (Channels, error) {
	log.Println("Reading channels")
	var channels Channels
	data, err := ioutil.ReadFile(ChannelsFile)
	if err != nil {
		if os.IsNotExist(err) {
			log.Println("Channels file does not exist, creating new")
			return make(Channels), nil
		}
		return channels, err
	}
	err = json.Unmarshal(data, &channels)
	return channels, err
}

// Write channels to the JSON file
func writeChannels(channels Channels) error {
	log.Println("Writing channels")
	data, err := json.MarshalIndent(channels, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(ChannelsFile, data, 0644)
}

// Send a success response back to Slack
func responseSuccess(w http.ResponseWriter, message string) {
	log.Printf("Sending success response: %s", message)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(&slack.Msg{Text: message})
}

// Send an error response back to Slack
func responseError(w http.ResponseWriter, message string) {
	log.Printf("Sending error response: %s", message)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(&slack.Msg{Text: message})
}
