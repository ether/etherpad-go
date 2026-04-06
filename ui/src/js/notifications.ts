type NotificationInput = {
  title?: string;
  text: string | Node;
  class_name?: string;
  sticky?: boolean;
  time?: number | string;
  position?: 'top' | 'bottom';
};

const active = new Map<string, number | null>();
let counter = 0;

const ensureContainer = (position: 'top' | 'bottom'): HTMLElement => {
  let container = document.querySelector<HTMLElement>(`#gritter-container.${position}`);
  if (container) return container;
  container = document.createElement('div');
  container.id = 'gritter-container';
  container.className = position;
  document.body.appendChild(container);
  return container;
};

const removeById = (id: string): void => {
  const item = document.getElementById(id);
  if (!item) return;
  const timer = active.get(id);
  if (typeof timer === 'number') window.clearTimeout(timer);
  active.delete(id);
  item.remove();
  if (document.querySelectorAll('#gritter-container .gritter-item').length === 0) {
    document.querySelectorAll('#gritter-container').forEach((node) => node.remove());
  }
};

const contentToNode = (text: string | Node): Node => {
  if (typeof text === 'string') {
    const span = document.createElement('span');
    span.textContent = text;
    return span;
  }
  return text;
};

export const notifications = {
  add(args: NotificationInput): string {
    const id = `gritter-item-${++counter}`;
    const position = args.position ?? 'top';
    const container = ensureContainer(position);

    const item = document.createElement('div');
    item.id = id;
    item.className = ['popup', 'gritter-item', args.class_name ?? ''].filter(Boolean).join(' ');

    const content = document.createElement('div');
    content.className = 'popup-content';

    if (args.title) {
      const title = document.createElement('h3');
      title.className = 'gritter-title';
      title.textContent = args.title;
      content.appendChild(title);
    }

    const body = document.createElement('div');
    body.className = 'gritter-content';
    body.appendChild(contentToNode(args.text));

    const close = document.createElement('div');
    close.className = 'gritter-close';
    close.innerHTML = '<button class="buttonicon buttonicon-cancel" aria-label="Close notification"></button>';
    close.addEventListener('click', () => removeById(id));

    content.append(body, close);
    item.append(content);
    container.appendChild(item);

    // Allow CSS transitions to pick up popup-show.
    requestAnimationFrame(() => item.classList.add('popup-show'));

    if (!args.sticky) {
      const ttl = Number(args.time ?? 3000);
      const timer = window.setTimeout(() => removeById(id), Number.isFinite(ttl) ? ttl : 3000);
      active.set(id, timer);
    } else {
      active.set(id, null);
    }

    return id;
  },

  remove(id: string): void {
    removeById(id);
  },

  removeAll(): void {
    Array.from(active.keys()).forEach((id) => removeById(id));
  },
};

export default notifications;
