package dav

import (
	"bytes"
	"context"
	"strings"

	"github.com/emersion/go-vcard"
	"github.com/emersion/go-webdav/carddav"
)

const defaultBookID = "default"

// NewCardDAVHandler builds a CardDAV HTTP handler backed by the store. prefix is
// the URL path the handler is mounted at (for example "/dav/carddav").
func NewCardDAVHandler(store *Store, prefix string) *carddav.Handler {
	return &carddav.Handler{Backend: &cardBackend{store: store, pathHelper: pathHelper{prefix: prefix}}, Prefix: prefix}
}

// cardBackend implements carddav.Backend on top of the SQLite store, scoping all
// data to the authenticated user taken from the request context.
type cardBackend struct {
	store *Store
	pathHelper
}

// CurrentUserPrincipal returns the principal URL for the authenticated user.
func (b *cardBackend) CurrentUserPrincipal(ctx context.Context) (string, error) {
	return b.principal(userFrom(ctx)), nil
}

// AddressBookHomeSetPath returns the collection holding the user's address books.
func (b *cardBackend) AddressBookHomeSetPath(ctx context.Context) (string, error) {
	return b.userHome(userFrom(ctx)), nil
}

func (b *cardBackend) toAddressBook(user string, bk Book) carddav.AddressBook {
	name := bk.Name
	if name == "" {
		name = "Contacts"
	}
	return carddav.AddressBook{
		Path:            b.collPath(user, bk.ID),
		Name:            name,
		Description:     bk.Description,
		MaxResourceSize: 1 << 20,
	}
}

// ListAddressBooks returns the user's address books, ensuring a default exists.
func (b *cardBackend) ListAddressBooks(ctx context.Context) ([]carddav.AddressBook, error) {
	user := userFrom(ctx)
	if err := b.store.EnsureBook(user, defaultBookID, "Contacts"); err != nil {
		return nil, err
	}
	books, err := b.store.ListBooks(user)
	if err != nil {
		return nil, err
	}
	out := make([]carddav.AddressBook, 0, len(books))
	for _, bk := range books {
		out = append(out, b.toAddressBook(user, bk))
	}
	return out, nil
}

// GetAddressBook returns a single address book.
func (b *cardBackend) GetAddressBook(ctx context.Context, path string) (*carddav.AddressBook, error) {
	user := userFrom(ctx)
	id, err := b.collID(user, path)
	if err != nil {
		return nil, err
	}
	bk, err := b.store.GetBook(user, id)
	if err != nil || bk == nil {
		return nil, err
	}
	ab := b.toAddressBook(user, *bk)
	return &ab, nil
}

// CreateAddressBook creates a new address book from the request.
func (b *cardBackend) CreateAddressBook(ctx context.Context, ab *carddav.AddressBook) error {
	user := userFrom(ctx)
	id, err := b.collID(user, ab.Path)
	if err != nil {
		return err
	}
	return b.store.CreateBook(user, Book{ID: id, Name: ab.Name, Description: ab.Description})
}

// DeleteAddressBook removes an address book.
func (b *cardBackend) DeleteAddressBook(ctx context.Context, path string) error {
	user := userFrom(ctx)
	id, err := b.collID(user, path)
	if err != nil {
		return err
	}
	return b.store.DeleteBook(user, id)
}

func (b *cardBackend) toObject(user, bookID string, o Object) (carddav.AddressObject, error) {
	card, err := vcard.NewDecoder(strings.NewReader(o.Data)).Decode()
	if err != nil {
		return carddav.AddressObject{}, err
	}
	return carddav.AddressObject{
		Path:          b.objectPath(user, bookID, o.UID, ".vcf"),
		ModTime:       o.Modified,
		ContentLength: int64(len(o.Data)),
		ETag:          o.ETag,
		Card:          card,
	}, nil
}

// GetAddressObject returns a single vCard object.
func (b *cardBackend) GetAddressObject(ctx context.Context, path string, _ *carddav.AddressDataRequest) (*carddav.AddressObject, error) {
	user := userFrom(ctx)
	bookID, uid, err := b.objectRef(user, path)
	if err != nil {
		return nil, err
	}
	o, err := b.store.GetObject(user, bookID, uid)
	if err != nil || o == nil {
		return nil, err
	}
	ao, err := b.toObject(user, bookID, *o)
	if err != nil {
		return nil, err
	}
	return &ao, nil
}

// ListAddressObjects returns every object in an address book.
func (b *cardBackend) ListAddressObjects(ctx context.Context, path string, _ *carddav.AddressDataRequest) ([]carddav.AddressObject, error) {
	user := userFrom(ctx)
	id, err := b.collID(user, path)
	if err != nil {
		return nil, err
	}
	objs, err := b.store.ListObjects(user, id)
	if err != nil {
		return nil, err
	}
	out := make([]carddav.AddressObject, 0, len(objs))
	for _, o := range objs {
		ao, err := b.toObject(user, id, o)
		if err != nil {
			return nil, err
		}
		out = append(out, ao)
	}
	return out, nil
}

// QueryAddressObjects lists objects and applies the client's filter.
func (b *cardBackend) QueryAddressObjects(ctx context.Context, path string, query *carddav.AddressBookQuery) ([]carddav.AddressObject, error) {
	all, err := b.ListAddressObjects(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	return carddav.Filter(query, all)
}

// PutAddressObject stores (creates or replaces) a vCard object.
func (b *cardBackend) PutAddressObject(ctx context.Context, path string, card vcard.Card, _ *carddav.PutAddressObjectOptions) (*carddav.AddressObject, error) {
	user := userFrom(ctx)
	bookID, uid, err := b.objectRef(user, path)
	if err != nil {
		return nil, err
	}
	if err := b.store.EnsureBook(user, bookID, "Contacts"); err != nil {
		return nil, err
	}

	// Normalize to vCard 4.0 so the stored card always has a VERSION and the
	// fields the encoder requires.
	vcard.ToV4(card)
	var buf bytes.Buffer
	if err := vcard.NewEncoder(&buf).Encode(card); err != nil {
		return nil, err
	}
	stored, err := b.store.PutObject(user, bookID, uid, buf.String())
	if err != nil {
		return nil, err
	}
	ao := carddav.AddressObject{
		Path:          b.objectPath(user, bookID, uid, ".vcf"),
		ModTime:       stored.Modified,
		ContentLength: int64(len(stored.Data)),
		ETag:          stored.ETag,
		Card:          card,
	}
	return &ao, nil
}

// DeleteAddressObject removes a vCard object.
func (b *cardBackend) DeleteAddressObject(ctx context.Context, path string) error {
	user := userFrom(ctx)
	bookID, uid, err := b.objectRef(user, path)
	if err != nil {
		return err
	}
	return b.store.DeleteObject(user, bookID, uid)
}
