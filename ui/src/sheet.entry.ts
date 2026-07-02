import { startSheetEditor } from './js/sheet/sheetEditor';
import { recordCurrentDoc } from './js/recentDocs';

recordCurrentDoc();

const root = document.getElementById('sheet-root');
if (root) {
  startSheetEditor(root);
}
