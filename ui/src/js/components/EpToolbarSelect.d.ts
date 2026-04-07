export interface ToolbarSelectOption {
  label: string;
  value: string;
}

export interface EpToolbarSelectElement extends HTMLElement {
  options: ToolbarSelectOption[];
  value: string;
}
