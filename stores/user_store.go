package stores

import (
	"encoding/json"
	"time"

	"github.com/go-redis/redis"
	"github.com/segmentio/ksuid"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	pe "wuyrush.io/pin/errors"
	md "wuyrush.io/pin/models"
)

const (
	bcryptCost                int = 8
	maxOptLockAttmpt          int = 3
	fieldNameUserID               = "ID"
	fieldNameUserIDType           = "IDType"
	fieldNameUserPasswdHash       = "Hash"
	fieldNameUserActive           = "Active"
	fieldNameUserCreationTime     = "CreationTime"
	keyUsersPendingActivation     = "usersPendingActivation"
)

// UserStore vends operation to manage user and secret
type UserStore interface {
	// Register registers the user but keep it inactive
	Register(u md.User) *pe.PinErr
	// Activate activates an inactive user via specified activation id
	Activate(aid string) *pe.PinErr
}

type RedisUserStore struct {
	DB                       *redis.Client
	UserPendingActivationFor time.Duration
}

type UserPendingActivation struct {
	ID           string
	ActivationID string
	CreationTime time.Time
}

func (r *RedisUserStore) Register(u md.User) *pe.PinErr {
	clog := log.WithField("userID", u.ID)
	hash, err := bcrypt.GenerateFromPassword([]byte(u.Passwd), bcryptCost)
	if err != nil {
		clog.WithError(err).Error("error creating user password hash")
		return pe.ErrServiceFailure("error processing user password").WithCause(err)
	}
	// TODO: test the correctness of redis transaction logic
	if _, err = r.DB.TxPipelined(func(p redis.Pipeliner) error {
		// check if user had already existed
		if id, err := p.HGet(u.ID, fieldNameUserID).Result(); err != nil {
			clog.Error("error checking the existence of user")
			return err
		} else if id != "" {
			return pe.ErrExisted("user being registered had already existed")
		}
		// save user details to DB and set expiry
		if _, err := p.HMSet(u.ID, map[string]interface{}{
			fieldNameUserID:           u.ID,
			fieldNameUserIDType:       string(u.IDType),
			fieldNameUserPasswdHash:   hash,
			fieldNameUserActive:       u.Active,
			fieldNameUserCreationTime: u.CreationTime,
		}).Result(); err != nil {
			clog.Error("error saving user details to redis")
			return err
		}
		if _, err := p.Expire(u.ID, r.UserPendingActivationFor).Result(); err != nil {
			clog.Error("error setting expiry of user blob")
			return err
		}
		// generate user activation identifier and set expiry
		kid, err := ksuid.NewRandom()
		if err != nil {
			clog.Error("error generating user activation identifier")
			return err
		}
		if _, err := p.Set(kid.String(), u.ID, r.UserPendingActivationFor).Result(); err != nil {
			clog.Error("error saving user actiavtion identifier")
			return err
		}
		// save user id to pending activation queue for activation worker to activate
		// compared to directly spinning up activation workflow goroutine this helps 1) rate limiting simultaneous
		// running activation workflows and 2) keep track of pending activation user record in case of
		// activation workflow failure
		pub, err := json.Marshal(UserPendingActivation{
			ID:           u.ID,
			ActivationID: kid.String(),
			CreationTime: u.CreationTime,
		})
		if err != nil {
			clog.Error("error marshalling pending user details")
			return err
		}
		if _, err := p.SAdd(keyUsersPendingActivation, pub).Result(); err != nil {
			clog.Error("error enqueueing user to pending activation queue")
			return err
		}
		return nil
	}); err != nil {
		msg := "error registering user"
		clog.WithError(err).Error(msg)
		switch v := err.(type) {
		case *pe.PinErr:
			return v
		default:
			return pe.ErrServiceFailure(msg).WithCause(err)
		}
	}
	return nil
}

func (r *RedisUserStore) Activate(aid string) *pe.PinErr {
	// TODO: test the correctness of redis transaction logic
	clog := log.WithField("activationID", aid)
	if _, err := r.DB.TxPipelined(func(p redis.Pipeliner) error {
		pub, err := p.Get(aid).Bytes()
		if err != nil {
			if err == redis.Nil {
				return pe.ErrNotFound("user pending activation had been expired and removed")
			}
			clog.Error("error getting pending user details from redis")
			return err
		}
		var pu UserPendingActivation
		if err := json.Unmarshal(pub, &pu); err != nil {
			clog.Error("error unmarshalling pending user details")
			return err
		}
		clog = clog.WithField("userID", pu.ID)
		// check whether the user had been expired or not
		if pu.CreationTime.Add(r.UserPendingActivationFor).Before(time.Now()) {
			return pe.ErrNotFound("user pending activation had been expired and removed")
		}
		// flip active bit
		if _, err := p.HMSet(pu.ID, map[string]interface{}{
			fieldNameUserActive: true,
		}).Result(); err != nil {
			clog.Error("error flipping user's activate bit")
			return err
		}
		// remove expiry on user details
		if ok, err := p.Persist(pu.ID).Result(); err != nil {
			clog.Error("error clearing user's expiry")
			return err
		} else if !ok {
			return pe.ErrNotFound("user pending activation had been expired and removed")
		}
		// remove activation identifier
		if _, err := p.Del(aid).Result(); err != nil {
			clog.Error("error deleting activation identifier")
			return err
		}
		return nil
	}); err != nil {
		msg := "error activating user"
		clog.WithError(err).Error(msg)
		switch v := err.(type) {
		case *pe.PinErr:
			return v
		default:
			return pe.ErrServiceFailure(msg).WithCause(err)
		}
	}
	return nil
}

func (r *RedisUserStore) Close() *pe.PinErr {
	return nil
}

// given the user activation workflow is rather simple we can place it here
func loopSendActivationCorrespondence() {
	/*
		get users pending activation from user store
		for u in users
			compose activation correspondence for u - the form of correspondence can vary, since a user's means of contact can vary(email/phone)
			send out the activation correspondence to u's means of contact
	*/
}
