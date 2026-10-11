package host

import "regexp"

var roomName = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{0,47}$`)
var topicName = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,47}$`)

func (h *Host) roomBroadcast(artifact string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, target := range h.views {
		if target.Artifact != artifact {
			continue
		}
		target.mu.Lock()
		rooms := []string{}
		for room := range target.rooms {
			rooms = append(rooms, room)
		}
		target.mu.Unlock()
		for _, room := range rooms {
			peers := []map[string]any{}
			for _, v := range h.views {
				if v.Artifact != artifact {
					continue
				}
				v.mu.Lock()
				if v.connected && v.rooms[room] {
					data := map[string]any{}
					for k, val := range v.presence {
						data[k] = val
					}
					peers = append(peers, map[string]any{"peer": v.ID, "by": h.Owner, "isMe": true, "sameTab": v.ID == target.ID, "kind": "viewer", "guest": false, "presence": data})
				}
				v.mu.Unlock()
			}
			target.emit(map[string]any{"type": "room_peers", "room": room, "peers": peers})
		}
	}
}
