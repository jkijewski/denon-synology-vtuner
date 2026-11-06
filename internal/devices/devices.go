// Package devices keeps one station catalog per receiver.
//
// A receiver identifies itself with its MAC address in the vTuner login request.
// Requests without a known receiver use the "default" list.
package devices

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"denon-synology-vtuner/internal/atomicfile"
	"denon-synology-vtuner/internal/catalog"
)

// DefaultKey is the list used when a request can't be tied to a receiver.
const DefaultKey = "default"

// MaxDevices limits the number of lists, because anyone on the network can send a login
// with a new MAC address. Receivers beyond the limit use the default list.
const MaxDevices = 32

// seenSaveInterval limits how often a repeat login from an unchanged address writes to disk.
const seenSaveInterval = time.Hour

// Device is one receiver.
type Device struct {
	Key       string    `json:"key"`
	Name      string    `json:"name"`
	FirstSeen time.Time `json:"firstSeen"`
	LastSeen  time.Time `json:"lastSeen"`
	LastIP    string    `json:"lastIp,omitempty"`
}

// Registry holds the receivers and their catalogs.
type Registry struct {
	dir     string
	mu      sync.Mutex
	devices map[string]*Device
	stores  map[string]*catalog.Store
	byIP    map[string]string
}

var keyRE = regexp.MustCompile(`^([0-9a-f]{12}|default)$`)

// NormalizeMAC turns "00:11:22:33:44:55", "00-11-22-33-44-55" or "001122334455" into "001122334455".
// It returns "" for anything that is not a MAC address.
func NormalizeMAC(mac string) string {
	m := strings.ToLower(strings.NewReplacer(":", "", "-", "", ".", "").Replace(strings.TrimSpace(mac)))
	if m == DefaultKey || !keyRE.MatchString(m) {
		return ""
	}
	return m
}

// ValidKey reports whether key names a receiver list.
func ValidKey(key string) bool { return keyRE.MatchString(key) }

// Open loads the registry from dir, creating the default list when missing.
func Open(dir string) (*Registry, error) {
	r := &Registry{dir: dir, devices: map[string]*Device{}, stores: map[string]*catalog.Store{}, byIP: map[string]string{}}
	if err := os.MkdirAll(filepath.Join(dir, "devices"), 0o750); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "devices.json")
	b, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err == nil {
		var list []*Device
		if err := json.Unmarshal(b, &list); err != nil {
			// The station lists are separate files, so a damaged registry must not stop
			// the service. Keep the damaged file for inspection and rebuild from the lists.
			log.Printf("%s is damaged (%v); rebuilding the receiver list from the station lists", path, err)
			if err := os.Rename(path, path+".damaged"); err != nil {
				return nil, err
			}
			if list, err = rebuildList(dir); err != nil {
				return nil, err
			}
		}
		for _, d := range list {
			if d != nil && ValidKey(d.Key) {
				r.devices[d.Key] = d
				if d.LastIP != "" {
					r.byIP[d.LastIP] = d.Key
				}
			}
		}
	}
	if _, ok := r.devices[DefaultKey]; !ok {
		now := time.Now()
		r.devices[DefaultKey] = &Device{Key: DefaultKey, Name: "Default (unknown receivers)", FirstSeen: now, LastSeen: now}
		if err := r.saveLocked(); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// rebuildList finds the receivers that have a station list file. Their names and last-seen
// data are lost, and the next login fills in the rest.
func rebuildList(dir string) ([]*Device, error) {
	entries, err := os.ReadDir(filepath.Join(dir, "devices"))
	if err != nil {
		return nil, err
	}
	now := time.Now()
	var list []*Device
	for _, e := range entries {
		key := strings.TrimSuffix(e.Name(), ".json")
		if key != e.Name() && key != DefaultKey && ValidKey(key) {
			list = append(list, &Device{Key: key, Name: "Receiver " + key[6:], FirstSeen: now})
		}
	}
	return list, nil
}

func (r *Registry) saveLocked() error {
	list := make([]*Device, 0, len(r.devices))
	for _, d := range r.devices {
		list = append(list, d)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Key < list[j].Key })
	b, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.Write(filepath.Join(r.dir, "devices.json"), append(b, '\n'), 0o600)
}

func (r *Registry) storeLocked(key string) (*catalog.Store, error) {
	if s, ok := r.stores[key]; ok {
		return s, nil
	}
	s, err := catalog.Open(filepath.Join(r.dir, "devices", key+".json"))
	if err != nil {
		return nil, err
	}
	r.stores[key] = s
	return s, nil
}

// Login records a receiver seen with mac from ip and returns its key.
// A new receiver starts with a copy of the default list.
func (r *Registry) Login(mac, ip string) (string, error) {
	key := NormalizeMAC(mac)
	if key == "" {
		return r.ForIP(ip), nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	d, ok := r.devices[key]
	if !ok {
		if len(r.devices) >= MaxDevices {
			log.Printf("receiver limit of %d reached: %s uses the list for its address", MaxDevices, key)
			return r.forIPLocked(ip), nil
		}
		def, err := r.storeLocked(DefaultKey)
		if err != nil {
			return "", err
		}
		s, err := r.storeLocked(key)
		if err != nil {
			return "", err
		}
		if err := s.Replace(def.Snapshot()); err != nil {
			delete(r.stores, key)
			return "", err
		}
		d = &Device{Key: key, Name: "Receiver " + key[6:], FirstSeen: now}
		r.devices[key] = d
	}
	changed := !ok || d.LastIP != ip || now.Sub(d.LastSeen) >= seenSaveInterval
	if d.LastIP != ip && r.byIP[d.LastIP] == key {
		delete(r.byIP, d.LastIP)
	}
	d.LastSeen, d.LastIP = now, ip
	if ip != "" {
		r.byIP[ip] = key
	}
	if !changed {
		return key, nil
	}
	return key, r.saveLocked()
}

// ForIP returns the receiver last seen at ip, or the default key.
func (r *Registry) ForIP(ip string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.forIPLocked(ip)
}

func (r *Registry) forIPLocked(ip string) string {
	if k, ok := r.byIP[ip]; ok {
		return k
	}
	return DefaultKey
}

// Store returns the catalog of key. Unknown keys return an error.
func (r *Registry) Store(key string) (*catalog.Store, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.devices[key]; !ok {
		return nil, os.ErrNotExist
	}
	return r.storeLocked(key)
}

// List returns all receivers, the default list first.
func (r *Registry) List() []Device {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Device, 0, len(r.devices))
	for _, d := range r.devices {
		out = append(out, *d)
	}
	sort.Slice(out, func(i, j int) bool {
		if (out[i].Key == DefaultKey) != (out[j].Key == DefaultKey) {
			return out[i].Key == DefaultKey
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// Rename sets a receiver's display name.
func (r *Registry) Rename(key, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.devices[key]
	name = strings.TrimSpace(name)
	if !ok {
		return os.ErrNotExist
	}
	if name == "" || len(name) > 100 {
		return errors.New("name must be 1 to 100 characters")
	}
	d.Name = name
	return r.saveLocked()
}

// Delete removes a receiver and its list. The default list can't be deleted.
func (r *Registry) Delete(key string) error {
	if key == DefaultKey {
		return errors.New("the default list can't be deleted")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.devices[key]
	if !ok {
		return os.ErrNotExist
	}
	if r.byIP[d.LastIP] == key {
		delete(r.byIP, d.LastIP)
	}
	delete(r.devices, key)
	delete(r.stores, key)
	if err := os.Remove(filepath.Join(r.dir, "devices", key+".json")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return r.saveLocked()
}
