import { createCsvTable, setupTheme } from "./table";
import "datatables.net-dt/css/dataTables.dataTables.css";
import "datatables.net-buttons-dt/css/buttons.dataTables.css";
import "datatables.net-scroller-dt/css/scroller.dataTables.css";
import "./table.css";

Object.assign(window, { CsvToTable: { createCsvTable, setupTheme } });
