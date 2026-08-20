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

export function setupTheme(selector: string) {
  const button = document.querySelector<HTMLButtonElement>(selector);
  if (!button) return;

  let theme: "light" | "dark" = "light";
  try {
    theme = localStorage.getItem(themeKey) === "dark" ? "dark" : "light";
  } catch {}

  const apply = (next: "light" | "dark") => {
    theme = next;
    document.documentElement.classList.toggle("dark", next === "dark");
    button.textContent = next === "dark" ? "☀ Light" : "☾ Dark";
    button.ariaLabel = `Use ${next === "dark" ? "light" : "dark"} theme`;
    button.title = button.ariaLabel;
    try {
      localStorage.setItem(themeKey, next);
    } catch {}
  };

  apply(theme);
  button.addEventListener("click", () => apply(theme === "dark" ? "light" : "dark"));
}

function addColumnFilters(table: any, data: CsvTableData) {
  table.columns().every(function (this: any) {
    const cell: HTMLElement | null = this.footer();
    if (!cell) return;

    const column = this;
    const index = column.index();
    const label = `Filter ${data.headers[index] ?? `column ${index + 1}`}`;
    const choices = new Set<string>();
    for (const row of data.rows) {
      if (row[index]) choices.add(row[index]);
      if (choices.size > filterChoiceLimit) break;
    }

    if (choices.size <= filterChoiceLimit) {
      const select = document.createElement("select");
      select.ariaLabel = label;
      select.add(new Option("All", ""));
      for (const choice of [...choices].sort((a, b) => a.localeCompare(b, undefined, { numeric: true }))) {
        select.add(new Option(choice, choice));
      }
      // An empty exact search matches only empty cells, so "All" clears the filter instead.
      select.addEventListener("change", () =>
        column.search(select.value, { exact: Boolean(select.value) }).draw(),
      );
      cell.replaceChildren(select);
      return;
    }

    const input = document.createElement("input");
    input.type = "search";
    input.placeholder = "Filter";
    input.size = 1; // let the column keep its content width instead of the input's default
    input.ariaLabel = label;
    let pending = 0;
    input.addEventListener("input", () => {
      clearTimeout(pending);
      pending = window.setTimeout(() => column.search(input.value).draw(), 150);
    });
    cell.replaceChildren(input);
  });
}

export function createCsvTable(selector: string, data: CsvTableData, options: CsvTableOptions) {
  const virtual =
    options.virtualScroll === 0 ||
    (options.virtualScroll > 0 && data.rows.length > options.virtualScroll);
  const lengthMenu = [...new Set([-1, 10, 25, 50, options.displayLength])].sort((a, b) => a - b);
  const exportButtons = (
    options.exportOptions.length ? options.exportOptions : ["copy", "csv", "json", "print", "colvis"]
  ).map((name) => (name === "colvis" ? { extend: "colvis", text: "Columns" } : name));
  const layout: Record<string, unknown> = {};
  const themeToggle = document.querySelector("#theme-toggle");
  const element = document.querySelector<HTMLTableElement>(selector);

  if (options.columnFilters && element) {
    const row = element.createTFoot().insertRow();
    for (const _ of data.headers) row.appendChild(document.createElement("th"));
  }

  if (options.exportEnabled) layout.topStart = { buttons: exportButtons };
  if (themeToggle) layout.topEnd = [{ search: { placeholder: "Search", text: "" } }, themeToggle];
  if (virtual) layout.bottomEnd = null;

  const table = new DataTable(selector, {
    columns: data.headers.map((title) => ({ title })),
    data: data.rows,
    deferRender: true,
    layout: Object.keys(layout).length ? layout : undefined,
    lengthMenu: [lengthMenu, lengthMenu.map((length) => (length === -1 ? "All" : length))],
    order: options.preserveSort ? [] : undefined,
    pageLength: virtual ? -1 : options.displayLength,
    paging: virtual || options.pagination,
    scrollX: true,
    scrollY: options.height === "auto" ? "50vh" : options.height,
    scroller: virtual,
  } as any);
  document.querySelector<HTMLInputElement>(".dt-search input")?.setAttribute("aria-label", "Search");
  if (options.columnFilters) addColumnFilters(table, data);

  if (options.height === "auto") {
    let frame = 0;
    const fit = () => {
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(() => {
        const container = table.table().container() as HTMLElement;
        const scrollBody = container.querySelector<HTMLElement>(".dt-scroll-body");
        if (!scrollBody) return;
        const parent = container.closest("main") ?? document.body;
        const bottomMargin = parseFloat(getComputedStyle(parent).marginBottom) || 0;
        const height = Math.max(
          120,
          scrollBody.getBoundingClientRect().height +
            window.innerHeight -
            container.getBoundingClientRect().bottom -
            bottomMargin,
        );
        scrollBody.style.height = `${Math.floor(height)}px`;
        scrollBody.style.maxHeight = scrollBody.style.height;
        table.columns.adjust();
      });
    };
    fit();
    window.addEventListener("resize", fit);
  }

  return table;
}
