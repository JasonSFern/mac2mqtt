# Mac2MQTT

`mac2mqtt` is a program that allows viewing and controlling some aspects of computers running macOS via MQTT.

This repo is a fork of bessarabov/mac2mqtt, that fixes bugs and adds features.

The following features have been added:

- mac2mqtt status publishing to MQTT every minute.
- display brightness control (requires brightness to be installed, can be installed via "brew install brightness")
- system logging for errors
- functions to retrieve the mac's model and serial number via system info
- changed the topic for system functions to use /set

It publishes to MQTT:

- current mac2mqtt status
- current volume
- volume mute state
- battery charge percent
- display brightness percent (requires homebrew package "brightness" to work).

You can send topics to:

- change display brightness
- change volume
- mute/unmute
- put the computer to sleep
- shutdown computer
- turn off the display
- wake display
- run macOS shortcuts

## Running

To run this program you need to put 2 files in a directory (`/Users/USERNAME/mac2mqtt/`):

    mac2mqtt
    mac2mqtt.yaml

Edit `mac2mqtt.yaml` (the sample file is in this repository), make binary executable (`chmod +x mac2mqtt`) and run `./mac2mqtt`:

    $ ./mac2mqtt
    2021/04/12 10:37:28 Started
    2021/04/12 10:37:29 Connected to MQTT
    2021/04/12 10:37:29 Sending 'true' to topic: mac2mqtt/${hostname}/status/alive

## Running in the background

You need `mac2mqtt.yaml` and `mac2mqtt` to be placed in the directory `/Users/USERNAME/mac2mqtt/`,
then you need edit the file `com.jasonsfern.mac2mqtt.plist`
and replace `USERNAME` with your username. Then put the file in `/Library/LaunchAgents/`.

And run:

    launchctl load /Library/LaunchAgents/com.jasonsfern.mac2mqtt.plist

(To stop you need to run `launchctl unload /Library/LaunchAgents/com.jasonsfern.mac2mqtt.plist`)

## Home Assistant sample config

![](https://user-images.githubusercontent.com/47263/114361105-753c4200-9b7e-11eb-833c-c26a2b7d0e00.png)

`configuration.yaml`:

```yaml
script:
  air2_sleep:
    icon: mdi:laptop
    sequence:
      - service: mqtt.publish
        data:
          topic: 'mac2mqtt/${hostname}/command/sleep'
          payload: 'sleep'

  air2_shutdown:
    icon: mdi:laptop
    sequence:
      - service: mqtt.publish
        data:
          topic: 'mac2mqtt/${hostname}/command/shutdown'
          payload: 'shutdown'

  air2_displaysleep:
    icon: mdi:laptop
    sequence:
      - service: mqtt.publish
        data:
          topic: 'mac2mqtt/${hostname}/command/displaysleep'
          payload: 'displaysleep'

mqtt:
  sensor:
    - name: air2_alive
      icon: mdi:laptop
      state_topic: 'mac2mqtt/${hostname}/status/alive'

    - name: 'air2_battery'
      icon: mdi:battery-high
      unit_of_measurement: '%'
      state_topic: 'mac2mqtt/${hostname}/status/battery'
```

## MQTT topics structure

The program is working with several MQTT topics. All topics are prefixed with `mac2mqtt` + `COMPUTER_NAME`.
For example, the topic with the current volume on my machine is `mac2mqtt/${hostname}/status/volume`

`mac2mqtt` send info to the topics `mac2mqtt/COMPUTER_NAME/status/#` and listen for commands in topics
`mac2mqtt/COMPUTER_NAME/command/#`.

### PREFIX + `/status/alive`

There can be `true` or `false` in this topic. If `mac2mqtt` is connected to MQTT server there is `true`.
If `mac2mqtt` is disconnected from MQTT there is `false`. This is the standard MQTT thing called Last Will and Testament.

### PREFIX + `/status/volume`

The value ranges from 0 (inclusive) to 100 (inclusive)—the current volume of the computer.

The value of this topic is updated every 60 seconds.

### PREFIX + `/status/mute`

There can be `true` or `false` in this topic. `true` means that the computer volume is muted (no sound),
`false` means that it is not muted.

### PREFIX + `/status/battery`

The value ranges from 0 (inclusive) to 100 (inclusive) and represents the current level of the battery. Returns empty if there is no battery.

The value of this topic is updated every 60 seconds.

### PREFIX + `/command/brightness`

You can send integer numbers from 0 (inclusive) to 100 (inclusive) to this topic. It will set the display brightness on the computer (requires homebrew package "brightness" to work).

### PREFIX + `/command/volume`

You can send integer numbers from 0 (inclusive) to 100 (inclusive) to this topic. It will set the volume on the computer.

### PREFIX + `/command/mute`

You can send `true` or `false` to this topic. When you send `true` the computer is muted. When you send `false` the computer
is unmuted.

### PREFIX + `/command/runshortcut`

You can send the name of a shortcut to this topic. It will run this shortcut in the Shortcuts app.

### PREFIX + `/command/set`

The following are the options that can be sent to `/set`. Any any values not part of this list will do nothing.

- `screensaver`: It will turn start your screensaver.
- `displaywake`: It will turn on the display.
- `sleep`: It will put the computer to sleep.
- `shutdown`: It will try to shut down the computer. The way it is done depends on the user who ran the program. If the program is run by `root` the computer will shut down, but if it is run by an ordinary user the computer will not shut down if there is another user who logged in.
- `displaysleep`: It will turn off the display.

## Building

To build this program yourself, follow these steps:

1. Clone this repo
2. Make sure you have installed go, for example with `brew install go`
3. Install its dependencies with `go install`
4. Build with `go build mac2mqtt.go`

It outputs a file `mac2mqtt`. Make the binary executable (`chmod +x mac2mqtt`) and run `./mac2mqtt`.
