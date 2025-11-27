import {SocketIoWrapper} from "./socketIoWrapper.ts";

declare global {
  interface Window { socket: SocketIoWrapper; socketio: {connect: Function} }
}

/**
 * Creates a socket.io connection.
 *     window.location.
 *     https://socket.io/docs/v2/client-api/#new-Manager-url-options
 * @return socket.io Socket object
 * @param namespace
 */
export const connect = () => {
  // The API for socket.io's io() function is awkward. The documentation says that the first
  // argument is a URL, but it is not the URL of the socket.io endpoint. The URL's path part is used
  // as the name of the socket.io namespace to join, and the rest of the URL (including query
  // parameters, if present) is combined with the `path` option (which defaults to '/socket.io', but
  // is overridden here to allow users to host Etherpad at something like '/etherpad') to get the
  // URL of the socket.io endpoint.

  window.socket = new SocketIoWrapper()

  window.socket.on('connect_error', (error: any) => {
    console.log('Error connecting to pad', error);
    /*if (socket.io.engine.transports.indexOf('polling') === -1) {
      console.warn('WebSocket connection failed. Falling back to long-polling.');
      socket.io.opts.transports = ['websocket','polling'];
      socket.io.engine.upgrade = false;
    }*/
  });


  return window.socket;
};
