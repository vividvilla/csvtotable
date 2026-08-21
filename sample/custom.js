/*
 * Sample --js for csvtotable.
 *
 *   csvtotable sample/meteorite-landings-1.csv out.html \
 *     --theme tokyonight \
 *     --css sample/custom.css \
 *     --js sample/custom.js
 *
 * This runs after the table has been built, so CsvToTable.table is the live
 * DataTables API instance: https://datatables.net/reference/api/
 *
 * All it does is announce itself. A <dialog> is used so the browser handles
 * focus and the Escape key, and the colours come from the page's theme
 * variables so the notice follows whichever theme is active.
 */
(() => {
  const dialog = document.createElement("dialog");
  dialog.style.cssText = `
    max-width: 32rem;
    padding: 1.25rem;
    border: 1px solid var(--ct-rule-strong);
    border-radius: 8px;
    background: var(--ct-paper);
    color: var(--ct-ink);
    font: inherit;
  `;
  dialog.innerHTML = `
    <h2 style="margin:0 0 .5rem">Custom JavaScript is running</h2>
    <p style="margin:0 0 1rem;color:var(--ct-muted)">
      This page was built with <code>--js sample/custom.js</code>. If you also
      passed <code>--css sample/custom.css</code>, pick <strong>Tokyonight</strong>
      from the theme menu to see the stylesheet it adds.
    </p>
    <form method="dialog" style="margin:0">
      <button style="font:inherit;padding:.4rem .8rem;border-radius:6px;
        border:1px solid var(--ct-rule-strong);background:var(--ct-wash);
        color:var(--ct-accent);cursor:pointer">Close</button>
    </form>
  `;

  document.body.append(dialog);
  dialog.showModal();
})();
