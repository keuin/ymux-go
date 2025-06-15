package yggdrasil

type Server interface {
	// HasJoined returns nil if and only if err != nil
	HasJoined(username string, serverID string) (*HasJoinedResponse, error)
	// Name returns a human-readable, unique name of this server
	Name() string
	// GetMinecraftProfiles implements: POST `/api/profiles/minecraft`
	GetMinecraftProfiles(usernames []string) (GetMinecraftProfilesResponse, error)
}

type HasJoinedResponse struct {
	StatusCode int    `json:"-"`
	RawBody    []byte `json:"-"`
	ServerName string `json:"-"`

	ProfileInfo
}

func (r HasJoinedResponse) HasJoined() bool {
	return r.StatusCode == 200 && r.ID != "" && r.Name != ""
}

type GetMinecraftProfilesResponse []ProfileInfo

type ProfileInfo struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Properties []ProfileProperty `json:"properties"`
}

type ProfileProperty struct {
	Name      string `json:"name"`
	Value     string `json:"value"`
	Signature string `json:"signature,omitempty"`
}
