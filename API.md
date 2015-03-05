## API Reference

#### Properties
We abstract commands out to the idea of properties. Things like the current power state, input devie, and volume level, are all properties that can either be set or retrieved. Thus, at this point, all API points that the gateway knows how to respond to are related to properties on the eISCP device.

##### Retrieving a property
We can retrieve the current state of the device, or specifically a property of it, via a GET request to /device/{PROPERTY}

Example
```
GET 127.0.0.1:3000/device/MVL
```
This request retrieves the master volume level of the device which the gateway is currently connected to. In this case, we as the client must know that the device will return a hexadecimal value, and interpret that for ourselves. Unlike the eiscp-intermediary, interpretation is up to us.

##### Setting a property
Setting a property of a device is just as simple as retrieving it. We POST to /device/{PROPERTY}/{VALUE}

Example
```
POST 127.0.0.1:3000/device/MVL/2A
```
This request tells the gateway to send a command requesting the device change the master volume level to 0x2A (42). Again, the interpretation must be done by the client before it gets to the gateway, we must know to take the desired volume level and convert it to hexidecmal format.

#### Meta-data

There are a few other API requests that the gateway can handle. 

```
GET 127.0.0.1:3000/status
```
Returns the current state of the gateway, including information regarding which device if any the gateway is connected to.

```
PUT 127.0.0.1:3000/device/{IP}/{PORT}
```
Tells the gateway to attempt an ISCP connection to the specified IP/PORT

```
DELETE 127.0.0.1:3000/device
```
Tells the gateway to disconnect from any currently connected device

```
* 127.0.0.1:3000/kill
```
Ends the gateway service or process
