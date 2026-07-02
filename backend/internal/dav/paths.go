package dav

import (
	"context"
	"errors"
	"path"
	"strings"
)

// errInvalidPath is returned when a DAV collection or object path cannot be
// parsed.
var errInvalidPath = errors.New("invalid DAV path")

// ctxKey is the private type for the authenticated DAV user stored in context.
type ctxKey struct{}

var userKey ctxKey

// WithUser returns a context carrying the authenticated DAV user (email).
func WithUser(ctx context.Context, user string) context.Context {
	return context.WithValue(ctx, userKey, user)
}

// userFrom extracts the authenticated user from the context.
func userFrom(ctx context.Context) string {
	u, _ := ctx.Value(userKey).(string)
	return u
}

// pathHelper builds and parses DAV URL paths for a handler mounted at prefix.
// It is shared by the CardDAV and CalDAV backends, which use identical path
// schemes ({prefix}/{user}/{collection}/{uid}{ext}).
type pathHelper struct {
	prefix string
}

func (p pathHelper) principal(user string) string { return p.prefix + "/principals/" + user + "/" }
func (p pathHelper) userHome(user string) string  { return p.prefix + "/" + user + "/" }
func (p pathHelper) collPath(user, id string) string {
	return p.userHome(user) + id + "/"
}
func (p pathHelper) objectPath(user, id, uid, ext string) string {
	return p.collPath(user, id) + uid + ext
}

// relParts returns the path segments below the user's home collection,
// tolerating paths supplied either with or without the mount prefix.
func (p pathHelper) relParts(user, pth string) []string {
	s := strings.Trim(strings.TrimPrefix(pth, p.prefix), "/")
	s = strings.Trim(strings.TrimPrefix(s, user), "/")
	if s == "" {
		return nil
	}
	return strings.Split(s, "/")
}

// collID parses a collection path into its collection id.
func (p pathHelper) collID(user, pth string) (string, error) {
	parts := p.relParts(user, pth)
	if len(parts) == 0 {
		return "", errInvalidPath
	}
	return parts[0], nil
}

// objectRef parses an object path into its collection id and object uid, dropping
// the resource file extension (.vcf/.ics).
func (p pathHelper) objectRef(user, pth string) (string, string, error) {
	parts := p.relParts(user, pth)
	if len(parts) < 2 {
		return "", "", errInvalidPath
	}
	last := parts[len(parts)-1]
	return parts[0], strings.TrimSuffix(last, path.Ext(last)), nil
}
