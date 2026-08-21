import DataTable from "datatables.net-dt";
import "datatables.net-buttons-dt";
import "datatables.net-buttons/js/buttons.html5.mjs";
import "datatables.net-buttons/js/buttons.print.mjs";
import "datatables.net-buttons/js/buttons.colVis.mjs";
import "datatables.net-scroller-dt";

export interface CsvTableData {
  headers: string[];
  rows: string[][];
}

export interface CsvTableOptions {
  displayLength: number;
  height: string;
  pagination: boolean;
  virtualScroll: number;
  preserveSort: boolean;
  exportEnabled: boolean;
  exportOptions: Array<"copy" | "csv" | "json" | "print" | "colvis">;
  columnFilters: boolean;
}

const buttons = (DataTable as any).ext.buttons;
const themeKey = "csvtotable-theme";
// A column with more distinct values than this gets a text box instead of a dropdown.
const filterChoiceLimit = 25;
// Stands in for the search box in the active-filter list, which is keyed by column.
const globalSearch = -1;
// Least of the viewport the table keeps when the heading and description are tall.
const minimumViewportShare = 0.6;

buttons.json = {
  text: "JSON",
  action(_event: Event, table: any) {
    const exported = table.buttons.exportData();
    const data = exported.body.map((row: string[]) =>
      Object.fromEntries(exported.header.map((header: string, index: number) => [header, row[index]])),
    );
    const url = URL.createObjectURL(new Blob([JSON.stringify(data, null, 2)], { type: "application/json" }));
    const link = document.createElement("a");
    link.href = url;
    link.download = `${document.title || "Table"}.json`;
    link.click();
    URL.revokeObjectURL(url);
  },
};

// A theme is a [data-theme] block of variables in the stylesheet, so switching
// is just swapping the attribute; "auto" removes it and lets the stylesheet's
// media query follow the system.
export function setupTheme(selector: string) {
  const picker = document.querySelector<HTMLSelectElement>(selector);
  if (!picker) return;

  const apply = (theme: string) => {
    if (theme === "auto") delete document.documentElement.dataset.theme;
    else document.documentElement.dataset.theme = theme;
  };

  let stored: string | null = null;
  try {
    stored = localStorage.getItem(themeKey);
  } catch {}
  // A stored choice wins over the page's built-in theme, but only if it names
  // one this page actually has.
  if (stored && [...picker.options].some((option) => option.value === stored)) {
    picker.value = stored;
    apply(stored);
  }

  picker.addEventListener("change", () => {
    apply(picker.value);
    try {
      localStorage.setItem(themeKey, picker.value);
    } catch {}
  });
}

function chip(key: string, value: string, onRemove: () => void) {
  const element = document.createElement("button");
  element.type = "button";
  element.className = "csvtotable-chip";
  element.ariaLabel = `Clear ${key} filter ${value}`;
  for (const [className, text] of [
    ["csvtotable-chip-key", key],
    ["csvtotable-chip-value", value],
    ["csvtotable-chip-remove", "×"],
  ]) {
    const part = document.createElement("span");
    part.className = className;
    part.textContent = text;
    element.append(part);
  }
  element.addEventListener("click", onRemove);
  return element;
}

function setupFilters(table: any, data: CsvTableData, summary: HTMLElement) {
  const controls = new Map<number, HTMLInputElement | HTMLSelectElement>();
  const searchBox = () => document.querySelector<HTMLInputElement>(".dt-search input");

  table.columns().every(function (this: any) {
    const cell: HTMLElement | null = this.footer();
    if (!cell) return;

    const column = this;
    const index = column.index();
    const choices = new Set<string>();
    for (const row of data.rows) {
      if (row[index]) choices.add(row[index]);
      if (choices.size > filterChoiceLimit) break;
    }

    let control: HTMLInputElement | HTMLSelectElement;
    if (choices.size <= filterChoiceLimit) {
      const select = document.createElement("select");
      select.add(new Option("All", ""));
      for (const choice of [...choices].sort((a, b) => a.localeCompare(b, undefined, { numeric: true }))) {
        select.add(new Option(choice, choice));
      }
      // An empty exact search matches only empty cells, so "All" clears the filter instead.
      select.addEventListener("change", () =>
        column.search(select.value, { exact: Boolean(select.value) }).draw(),
      );
      control = select;
    } else {
      const input = document.createElement("input");
      input.type = "search";
      input.placeholder = "Filter";
      input.size = 1; // let the column keep its content width instead of the input's default
      let pending = 0;
      input.addEventListener("input", () => {
        clearTimeout(pending);
        pending = window.setTimeout(() => column.search(input.value).draw(), 150);
      });
      control = input;
    }
    control.ariaLabel = `Filter ${data.headers[index] ?? `column ${index + 1}`}`;
    controls.set(index, control);
    cell.replaceChildren(control);
  });

  const reset = (index: number) => {
    const control = controls.get(index);
    if (control) control.value = "";
    if (index !== globalSearch) return table.column(index).search("");
    const box = searchBox();
    if (box) box.value = "";
    return table.search("");
  };

  const render = () => {
    const active: Array<[number, string]> = [];
    if (table.search()) active.push([globalSearch, table.search()]);
    table.columns().every(function (this: any) {
      if (this.search()) active.push([this.index(), this.search()]);
    });

    summary.replaceChildren();
    summary.hidden = active.length === 0;
    for (const [index, value] of active) {
      const key = index === globalSearch ? "search" : (data.headers[index] ?? `column ${index + 1}`);
      summary.append(chip(key, value, () => reset(index).draw()));
    }
    if (active.length < 2) return;

    const clearAll = document.createElement("button");
    clearAll.type = "button";
    clearAll.className = "csvtotable-clear";
    clearAll.textContent = "Clear all";
    clearAll.addEventListener("click", () => {
      for (const index of [globalSearch, ...controls.keys()]) reset(index);
      table.draw();
    });
    summary.append(clearAll);
  };

  table.on("draw", render);
  render();
}

export function createCsvTable(selector: string, data: CsvTableData, options: CsvTableOptions) {
  const virtual =
    options.virtualScroll === 0 ||
    (options.virtualScroll > 0 && data.rows.length > options.virtualScroll);
  const lengthMenu = [...new Set([-1, 10, 25, 50, options.displayLength])].sort((a, b) => a - b);
  const chosen = options.exportOptions.length
    ? options.exportOptions
    : ["copy", "csv", "json", "print", "colvis"];
  const exports = chosen.filter((name) => name !== "colvis");
  const toolbar: unknown[] = [];
  // Exports are rare, so they collapse behind one menu rather than a row of buttons.
  if (exports.length > 1) toolbar.push({ extend: "collection", text: "Export", buttons: exports });
  else if (exports.length) toolbar.push(exports[0]);
  if (chosen.length !== exports.length) toolbar.push({ extend: "colvis", text: "Columns" });

  const element = document.querySelector<HTMLTableElement>(selector);
  const themeToggle = document.querySelector(".csvtotable-theme");
  const summary = document.createElement("div");
  summary.className = "csvtotable-filters";
  summary.hidden = true;

  if (options.columnFilters && element) {
    const row = element.createTFoot().insertRow();
    for (const _ of data.headers) row.appendChild(document.createElement("th"));
  }

  // Showing every row means one page, so the page controls would only ever be a
  // lone disabled "1".
  const paged = !virtual && options.pagination && options.displayLength > 0;
  const layout: Record<string, unknown> = {
    topStart: [{ search: { placeholder: "Search all columns", text: "" } }, "info"],
    topEnd: [...(options.exportEnabled ? [{ buttons: toolbar }] : []), ...(themeToggle ? [themeToggle] : [])],
    bottomStart: paged ? "pageLength" : null,
    bottomEnd: paged ? "paging" : null,
  };

  const table = new DataTable(selector, {
    columns: data.headers.map((title) => ({ title })),
    data: data.rows,
    deferRender: true,
    infoCallback: (_settings: any, _start: number, _end: number, max: number, total: number) =>
      `${total.toLocaleString()} ${total === 1 ? "row" : "rows"}` +
      (total === max ? "" : ` of ${max.toLocaleString()}`),
    language: {
      lengthMenu: "Rows per page _MENU_",
      zeroRecords: "No matching rows",
    },
    layout,
    lengthMenu: [lengthMenu, lengthMenu.map((length) => (length === -1 ? "All" : length))],
    order: options.preserveSort ? [] : undefined,
    pageLength: virtual ? -1 : options.displayLength,
    paging: virtual || options.pagination,
    scrollX: true,
    scrollY: options.height === "auto" ? "50vh" : options.height,
    scroller: virtual,
  } as any);
  document.querySelector<HTMLInputElement>(".dt-search input")?.setAttribute("aria-label", "Search all columns");

  const container = table.table().container() as HTMLElement;
  container.querySelector(".dt-layout-row")?.after(summary);
  if (options.columnFilters) setupFilters(table, data, summary);

  if (options.height === "auto") {
    let frame = 0;
    const fit = () => {
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(() => {
        const scrollBody = container.querySelector<HTMLElement>(".dt-scroll-body");
        if (!scrollBody) return;
        const parent = container.closest("main") ?? document.body;
        const bottomMargin = parseFloat(getComputedStyle(parent).paddingBottom) || 0;
        const bodyRect = scrollBody.getBoundingClientRect();
        // Measured against the document, not the viewport, so a scrolled page
        // (which a long description causes) does not inflate the result.
        const trailing = container.getBoundingClientRect().bottom - bodyRect.bottom;
        const available =
          window.innerHeight - (bodyRect.top + window.scrollY) - trailing - bottomMargin;
        // A tall heading must not squeeze the table into a sliver; past this
        // point the page scrolls instead.
        const height = Math.max(120, window.innerHeight * minimumViewportShare, available);
        scrollBody.style.height = `${Math.floor(height)}px`;
        scrollBody.style.maxHeight = scrollBody.style.height;
        table.columns.adjust();
      });
    };
    fit();
    window.addEventListener("resize", fit);
    // Filter chips appear and disappear above the table, changing how much room
    // is left for it.
    table.on("draw", fit);
  }

  return table;
}
