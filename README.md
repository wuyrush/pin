# pin

`pin` is a micro web service for users to save and share information(primarily text) in limited time period.

## Requirements:

### Core Functionalities

1. A user can save the information with `pin`, aka "pin"ing the information
    1. the only supported information format currently is text (utf-8 encoded)
    1. the size of information to be pinned is limited up to 1 MiB.
1. Once the user saves the information, she gets back a url referring to the saved information;
    1. the url, along with the information referred by the url will expire after a limited but configurable time period;
        1. ALL the url and the information MUST expire after a hard limited time period, since this service doesn't mean to serve as a permenant storage for anybody.
    1. the user can specify whether the url is *public accessible* or *private*:
        1. *public accessible*: anyone with the url can access the information referred by the url, provided the url doesn't expire;
        1. *private*: only the user herself can access the url, while all other persons visiting the url will be rejected for access, provided the url doesn't expire.
    1. the user can specify time-to-expiry for the information to be pinned:
        1. By default, the pinned information expires in 30 minutes starting from the point it is pinned;
        1. The user can specify time-to-expiry from minimum 1 minute to maximum 1 day.
    1. the user can specify expiration condition of the information to be pinned based on the viewer count:
        1. By default, the information has NO expiration condition associated with its viewer count;
        1. The user can specify the information expires on minimum 1 to maximum 512 viewer count.
1. All pinned information cannot be modified once pinned;
1. User can delete a pinned information belong to her.

### Auth

1. ALL users can use `pin` in anonymous mode(aka without registration or logging in). The corresponding constraints are as follow:
    1. ALL the information pinned in anonymous mode is public accessible;
    1. ALL the information pinned in anonymous mode MUST expires in 30 minutes, starting from the point it is pinned. A user can still configure the time-to-expiry to a value between 1 and 30 minutes. 
    1. ALL the information pinned in anonymous mode cannot be removed by ANY users until it expires.
1. A user can register with `pin` in order to:
    1. retrieve a list of her pinned information;
    1. configure longer expiration time for information to be pinned;
    1. delete any pinned information belong to her.
1. Necessary information to finish user register workflow are:
    1. user's email address
    1. password chosen by user
1. A user needs her email address and password in order to login.
1. A user can reset her password via her email in case she lost the password.

### Legal related

The platform bears no liability to the information temporarily stored on `pin` and its consequence.

