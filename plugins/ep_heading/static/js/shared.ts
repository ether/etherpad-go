import type { ContentCollectorHook } from '../../../typings/etherpad';

const tags = ['h1', 'h2', 'h3', 'h4', 'code'] as const;

export const collectContentPre: ContentCollectorHook = (_hookName, context, cb) => {
  const {lineAttributes} = context.state;
  const tagIndex = tags.indexOf(context.tname as (typeof tags)[number]);

  if (context.tname === 'div' || context.tname === 'p') delete lineAttributes.heading;
  if (tagIndex >= 0) lineAttributes.heading = tags[tagIndex];

  return cb();
};

export const collectContentPost: ContentCollectorHook = (_hookName, context, cb) => {
  const {lineAttributes} = context.state;
  const tagIndex = tags.indexOf(context.tname as (typeof tags)[number]);
  if (tagIndex >= 0) delete lineAttributes.heading;
  return cb();
};
