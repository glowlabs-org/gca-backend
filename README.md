# GCA Backend

This repository contains all of the code related to running the GCA servers,
and the clients that report to the servers.

Run tests with `go test -v -tags test ./...`

To run a subset of tests, use the following:
```
go test -v -tags test -run TestRateLimiter ./... # runs all RateLimiter tests
```

## Standards and Conventions

Within the code, "PublicKey" can be abbreviated to "PubKey" or "pubKey", but
not "pubkey". Same convention goes for "PrivateKey" - "PrivKey" and "privKey"
are okay but "privkey" is not.

Filenames use camelCase and any non-standard filetype ends with the extention
'.dat'. For example, filenames can be 'gcaPubKey.dat' or 'history.dat'

Units:  
	+ 3 decimals of precision for latitude and longitude  
	+ Milliwatthours is the base unit of precision for energy  
	+ Milliwatts is the base unit of precision for power  
	+ Grams is the base unit of precision for emissions  
	+ Grams per megawatthour is the base unit of precision for emission rates  
	+ Protocol Fees are expressed in cents
	+ Timeslots (5 minutes) is the base unit of precision for protocol time  
	+ Seconds is the base unit of precision for clock time  
	+ Protocol epochs are often called 'buckets' and last one week  
	+ Within the context of Glow, "one year" is actually exactly 52 weeks, or 364 days  

Always use LittleEndian when encoding numbers to binary.

Anything that needs to be signed should have an explicit 'SigningBytes()'
function.

Comments within the code about how long a duration is or how frequently an
event happens may be out of date. The durations get tweaked all the time
because there wasn't any formal system in place for deciding how to choose
durations and frequencies for different tasks. They got tweaked and adjusted
kinda arbitrarily, in the hopes that the tweaks were improving things.

## Concurrency

Objects that can be used in concurrent contexts will have a mutex inside of
them. Only one mutex is allowed per object. The mutex protects all fields
inside of the object, unless one of the fields is prefixed with 'static'. A
'static' prefix indicates that the field is not allowed to change, and
therefore can be read without holding the mutex. Static fields can be modified
during construction, before the object enters into a concurrent context.

If there is an object inside of an object without a mutex, that whole object is
protect by the mutex of its parent. If an object inside of an object has a
mutex, the inner object is not protected by the mutex of the parent.

Mutexes are not allowed to stack. At no point in time should two mutexes be
held at once.

Mutexes must be locked and unlocked from the same context. It is not okay to
lock a mutex, then call a function which will unlock that mutex on its own. If
a mutex is locked, the reviewer must be able to see the location where the
unlock occurs within the same file and must be able to appraise the correctness
of the unlock without reading any other code.

Exported methods of objects are assumed to require locking a mutex. Therefore,
the implementation of these methods cannot assume that the caller is protecting
them against race conditions. Further, callers must assume that the exported
method will hold a mutex, therefore the caller cannot beholding a mutex
themselves when calling exported methods.

Unexported methods of objects default to assuming that they will be called with
a mutex already held. They can freely access the state of their object under
the assumption that the caller is already holding the lock.

Unexported methods of objects prefixed by 'managed' will manage their own
mutex. This means that they cannot be called while a mutex is held.

Methods of an object prefixed by 'static' do not require the mutexes of the
object in any way, and therefore are safe to call under any circumstances.

Following all of the above concurrency rules is guaranteed to prevent race
conditions and deadlocks. And though they may feel strict to someone who is not
used to them, they actually are very expressive and allow the programmer to
design nearly anything. The conventions just take a bit of familiarity to use
effecively.

Code must take care not to panic, fatal, or exit while a mutex is being held,
because this can cause a deadlock when any deferred functions are called.
Programmers experienced with using these conventions have 95% or more of their
deadlocks occur due to calling either panic() or t.Fatal() while holding a
mutex and getting stuck as a deferred Close() call tries to grab the same
mutex.

The glow.SafeMu object can be used during debugging to locate the source of a
deadlock. It's not meant to be used in production because it incurs reasonably
meaningful computational overhead.

## Coordination

Any struct that has background threads should use a channel called 'closed'
which gets closed when `Close()` is called on the object. That channel signals
the background threads to shut down.

Avoid having a similar channel to synchronize threads that need to complete
some startup tasks. Instead, create a new channel for those threads before they
are spun up, and make sure that those threads have finished starting up before
placing the object in a concurrent environment.

The more general principle at play here is to make sure that the users of an
object do not have to worry about synchronization around the object. The New()
function and Close() function should be the extent of management that is
necessary.

## Untrusted Data

Any data that is received from an untrusted source must be tagged as untrusted
in the variable name. The data can only be passed to other functions that are
tagged as 'untrusted' until one of the functions fully cleans the data and
ensures that it is not corrupt.

The 'untrusted' tag for a function would come after any concurrency names. For
example, if you had a managed and untrusted function named integrateInput, the
full name would be managedUntrustedIntegrateInput, and not
untrustedManagdIntegrateInput. Generally speaking however you should not be
mixing the two, it is much better to sanitize and verify data before thrusting
it into a concurrent environment.

Data that is loaded from disk is typically considered to be trusted, unless it
is being loaded from a public source (like 'Downloads') and the user/program
has no way of knowing whether the data is valid. But programs should generally
be able to trust data on disk that they either wrote themselves or when those
files are naturally part of the program.

## API Structure

Every API response should have a signature from the GCA server that
authenticates it. This is especially important because data is being
transferred over http rather than https, therefore authentication needs to be
on everything.

Signatures happen by calling the SigningBytes() function. All SigningBytes()
functions should add a prefix with the struct name to minimize the chance for
replay attacks. Any data that might only be valid for a certain period of time
should have a timestamp attached to it.

## Assumptions

The glow-monitor assumes that there will be at least 30 minutes of network
uptime every day during the daylight, with at most 2 consecutive days of
downtime. More downtime than that will result in missing power reports that
under-represent a solar farms contributions.

## File Writing and Archiving

To ensure consistency of files stored on the server, they should be written
append-only, using a single write call. This allows consistent read access
without the need for file locking.

The archive strategy is to return all public data as files, providing
them in a zip archive. In case of updates during the archive
process, files must be archived in the reverse order to which they would
be modified. The archive order is: all device stats, equipment reports,
equipment authizations, gca public keys, and gca server public keys.
