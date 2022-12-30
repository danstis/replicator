package utility

import "github.com/diamondburned/arikawa/v3/discord"

// UserInList will return true if the provided UserID is in the provided list of user IDs.
func UserInList(userID discord.UserID, idList []discord.UserID) bool {
	for _, v := range idList {
		if v == userID {
			return true
		}
	}
	return false
}
