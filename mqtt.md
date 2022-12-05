# MQTT topics and message payloads

### Command station

   ***
#### Enable DCC output
    Event topic:
    "<topic root>/cs/<command station name>/enabled"
    
    Command topics:
    "<topic root>/cs/<command station name>/enabled/get"
    "<topic root>/cs/<command station name>/enabled/set"
    
    Payload: true | false

### Loco

   ***
#### Loco direction
    Event topic:
    "<topic root>/loco/<loco name>/dir"
    
    Command topics:
    "<topic root>/loco/<loco name>/dir/get"
    "<topic root>/loco/<loco name>/dir/set"
    "<topic root>/loco/<loco name>/dir/toggle"

    Payload: true | false

    true  := forward  direction
    false := backward direction

   ***
#### Loco speed
    Event topic:
    "<topic root>/loco/<loco name>/speed"

    Command topics:
    "<topic root>/loco/<loco name>/speed/get"
    "<topic root>/loco/<loco name>/speed/set"
        
    Payload: number
    
    number := speed range 0..126
    
    Command topic:
    "<topic root>/loco/<loco name>/speed/stop"

    Payload: none

    Emergency stop - the loco is stopped immediately ignoring deceleration settings 

   ***
#### Loco function
    Event topic:
    "<topic root>/loco/<loco name>/<loco function>
    
    Command topics:
    "<topic root>/loco/<loco name>/<loco function>/get"
    "<topic root>/loco/<loco name>/<loco function>/set"
    "<topic root>/loco/<loco name>/<loco function>/toggle"

    Payload: true | false

    true  := function on
    false := function off
