export class SocketIoWrapper {
    private socket: WebSocket
    private eventCallbacks: { [key: string]: Function[] } = {}

    constructor() {
        this.socket = new WebSocket('ws://localhost:3000/socket.io/')
        this.socket.onopen = () => {
            console.log('onopen')
            const iID = window.setInterval(() => {
                console.log('check')
                if (this.eventCallbacks['connect'] && this.eventCallbacks['connect'].length == 1) {
                    console.log('Handled connect event')
                    this.eventCallbacks['connect'].forEach(callback => {
                        callback()
                    })
                    clearInterval(iID)
                }
            }, 200)
        }
        this.socket.onmessage = (e)=>{
            const arr = JSON.parse(e.data)
            this.eventCallbacks[arr[0]].forEach(f=>{
                f(arr[1])
            })
        }
    }


    public connect() {
        console.log('connect')
    }

    public io = {
        on: (event: string, callback: Function)=>{
            this.on(event, callback)
        }
    }

    public once(event: string, callback: Function) {
        if (this.eventCallbacks[event]) {
            this.eventCallbacks[event].push(callback)
        } else {
            this.eventCallbacks[event] = [callback]
        }
    }

    public on(event: string, callback: Function) {
        console.log(event)
        if (this.eventCallbacks[event]) {
            this.eventCallbacks[event].push(callback)
        } else {
            this.eventCallbacks[event] = [callback]
        }
    }

    public emit(event: string, data: any) {
        this.socket.send(JSON.stringify({event, data}))
    }

    public off() {
        console.log("Off")
    }
}