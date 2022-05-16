package memory

import (
	"container/list"
	"sync"
	"time"

	"github.com/astaxie/session"
)

var provider = &Provider{list: list.New()}

type SessionStore struct {
	sid          string                      // unique session id
	timeAccessed time.Time                   // last access time
	value        map[interface{}]interface{} // session value stored inside
}

func (st *SessionStore) Set(key, value interface{}) error {
	st.value[key] = value
	provider.SessionUpdate(st.sid)
	return nil
}

func (st *SessionStore) Get(key interface{}) interface{} {
	provider.SessionUpdate(st.sid)
	if v, ok := st.value[key]; ok {
		return v
	}
	return nil
}

func (st *SessionStore) Delete(key interface{}) error {
	delete(st.value, key)
	provider.SessionUpdate(st.sid)
	return nil
}

func (st *SessionStore) SessionID() string {
	return st.sid
}

type Provider struct {
	lock     sync.Mutex               // lock
	sessions map[string]*list.Element // save in memory
	list     *list.List               // gc
}

func (provider *Provider) SessionInit(sid string) (session.Session, error) {
	provider.lock.Lock()
	defer provider.lock.Unlock()
	v := make(map[interface{}]interface{}, 0)
	newsess := &SessionStore{sid: sid, timeAccessed: time.Now(), value: v}
	element := provider.list.PushBack(newsess)
	provider.sessions[sid] = element
	return newsess, nil
}

func (provider *Provider) SessionRead(sid string) (session.Session, error) {
	if element, ok := provider.sessions[sid]; ok {
		return element.Value.(*SessionStore), nil
	}
	sess, err := provider.SessionInit(sid)
	return sess, err
}

func (provider *Provider) SessionDestroy(sid string) error {
	if element, ok := provider.sessions[sid]; ok {
		delete(provider.sessions, sid)
		provider.list.Remove(element)
	}
	return nil
}

func (provider *Provider) SessionGC(maxlifetime int64) {
	provider.lock.Lock()
	defer provider.lock.Unlock()

	for {
		element := provider.list.Back()
		if element == nil {
			break
		}
		if (element.Value.(*SessionStore).timeAccessed.Unix() + maxlifetime) < time.Now().Unix() {
			provider.list.Remove(element)
			delete(provider.sessions, element.Value.(*SessionStore).sid)
		} else {
			break
		}
	}
}

func (provider *Provider) SessionUpdate(sid string) error {
	provider.lock.Lock()
	defer provider.lock.Unlock()
	if element, ok := provider.sessions[sid]; ok {
		element.Value.(*SessionStore).timeAccessed = time.Now()
		provider.list.MoveToFront(element)
		return nil
	}
	return nil
}

func init() {
	provider.sessions = make(map[string]*list.Element, 0)
	session.Register("memory", provider)
}
