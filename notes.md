= dreamtv =

All in one tool for sharing a fake TV with others that streams video files according to a schedule.

== front end ==

=== def ===

- password protected
- single html page
- embedded player
- channel controls, both GUI and keyboard
- live, anonymous(?) chat
- program guide

=== maybe === 

- vfx picker
- themes (gotta have N I T E  M O D E)
- remote control passed randomly to viewer

== back end == 

- video files sorted into "channel" directory
- Awareness of whether viewers are connected or not (0 or N viewers)
- Ability to receive channel changing input
- basic websocket chat

=== HTTP endpoints ===

- sync flv `/stream` the actual HTTP wrapped flv stream
- async json `/schedule` json dump of the next 24 or 48 hours of scheduling
- async `/chanup` move to next channel
- async `/chandown` move to previous channel
- sync html `/` serve player page
- `/register` connect a websocket for a viewer

=== scheduling pseudocode ===

Given that: 
- there's a sqlite DB
- there are directories with names like "action" "horror" "howto" "comedy" "commercials" full of flv
- play stats are tracked for every file (play count, last played)
- if a file disappears, it's pruned from the DB

Every 24 hours, each directory is examined and a schedule is produced per-directory. Videos are
selected based on least-frequently-seen and commercials are selected to go in a block between each
channel (using a similar least-frequently-seen algorithm).

a schedule is produced and cached for the 24 hour period (or longer) and made available at an HTTP
endpoint.

Once this is done, given a timestamp T and channel C we can compute:

- what video file should be playing based on the schedule for C
- what offset into the video file we should seek to

We'll use this to start the stream when we go from 0 -> 1 viewers.
