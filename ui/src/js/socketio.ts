import {SocketIoWrapper} from "../socketIoWrapper.ts";

declare global {
  interface Window { socket: SocketIoWrapper; socketio: {connect: Function} }
}

/**
 * Creates a socket.io connection.
 * @param etherpadBaseUrl - Etherpad URL. If relative, it is assumed to be relative to
 *     window.location.
 * @param _namespace
 * @param options - socket.io client options. See
 *     https://socket.io/docs/v2/client-api/#new-Manager-url-options
 * @return socket.io Socket object
 */
const connect = (_etherpadBaseUrl: string | URL, _namespace = '/', _options = {}) => {
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

if (typeof exports === 'object') {
  exports.connect = connect;
} else {
  window.socketio = {connect};
}
