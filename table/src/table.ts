import DataTable from "datatables.net-dt";
import "datatables.net-buttons-dt";
import "datatables.net-buttons/js/buttons.html5.mjs";
import "datatables.net-buttons/js/buttons.print.mjs";
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
  exportOptions: Array<"copy" | "csv" | "json" | "print">;
}

const buttons = (DataTable as any).ext.buttons;
const themeKey = "csvtotable-theme";
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

export function createCsvTable(selector: string, data: CsvTableData, options: CsvTableOptions) {
  const virtual =
    options.virtualScroll === 0 ||
    (options.virtualScroll > 0 && data.rows.length > options.virtualScroll);
  const lengthMenu = [...new Set([-1, 10, 25, 50, options.displayLength])].sort((a, b) => a - b);
  const exportButtons = options.exportOptions.length
    ? options.exportOptions
    : ["copy", "csv", "json", "print"];
  const layout: Record<string, unknown> = {};
  const themeToggle = document.querySelector("#theme-toggle");

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
