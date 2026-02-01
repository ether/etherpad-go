const tags = ['h1', 'h2', 'h3', 'h4', 'code'];

export const collectContentPre = (hookName, context, cb) => {
  const tname = context.tname;
  const state = context.state;
  const lineAttributes = state.lineAttributes;
  const tagIndex = tags.indexOf(tname);
  if (tname === 'div' || tname === 'p') {
    delete lineAttributes.heading;
  }
  if (tagIndex >= 0) {
    lineAttributes.heading = tags[tagIndex];
  }
  return cb();
};

// I don't even know when this is run..
export const collectContentPost = (hookName, context, cb) => {
  const tname = context.tname;
  const state = context.state;
  const lineAttributes = state.lineAttributes;
  const tagIndex = tags.indexOf(tname);
  if (tagIndex >= 0) {
    delete lineAttributes.heading;
  }
  return cb();
};