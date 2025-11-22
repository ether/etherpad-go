import {connect} from "./socketio.ts";

const settingSocket = connect(`settings`);

export default settingSocket