# Chart Examples for CSVtoTable

This document provides examples of how to use the new chart visualization features in csvtotable.

## Basic Usage

### Pie Chart
Create a pie chart to show distribution or market share:

```bash
csvtotable market_share.csv output.html \
  --chart pie \
  --chart-labels "Company" \
  --chart-values "Market Share" \
  --chart-title "Market Share Distribution"
```

### Line Chart
Create a line chart to show trends over time:

```bash
csvtotable sales_data.csv output.html \
  --chart line \
  --chart-x "Month" \
  --chart-y "Revenue,Expenses" \
  --chart-title "Monthly Financial Trends"
```

### Bar Chart
Create a bar chart to compare values:

```bash
csvtotable product_sales.csv output.html \
  --chart bar \
  --chart-x "Product" \
  --chart-y "Units Sold" \
  --chart-title "Sales by Product"
```

## Advanced Usage

### Multiple Y-Axis Series
Create a line chart with multiple data series:

```bash
csvtotable sales_data.csv output.html \
  --chart line \
  --chart-x "Month" \
  --chart-y "Revenue,Expenses,Profit" \
  --chart-title "Complete Financial Overview"
```

### Multiple Charts
Add multiple charts to a single HTML file:

```bash
csvtotable sales_data.csv dashboard.html \
  --chart line \
  --chart-x "Month" \
  --chart-y "Revenue" \
  --chart-title "Revenue Trend" \
  --chart bar \
  --chart-x "Month" \
  --chart-y "Profit" \
  --chart-title "Monthly Profit"
```

### Auto-Generated Charts
Let csvtotable automatically detect and create appropriate charts:

```bash
csvtotable data.csv output.html --auto-charts
```

This will analyze your CSV data and automatically generate:
- Bar charts for categorical vs numeric data
- Line charts for time-series data
- Pie charts for distribution data (when applicable)

## Tips

1. **Multiple Y-values**: Separate column names with commas (no spaces) when using `--chart-y`
2. **Column Names with Spaces**: Use quotes around column names that contain spaces
3. **Chart Order**: Charts appear in the order you specify them in the command
4. **Responsive Design**: All charts are responsive and adapt to different screen sizes
5. **Data Requirements**: 
   - Pie charts need exactly 2 columns (labels and values)
   - Bar/Line charts need at least 1 X column and 1 or more Y columns
   - Y columns must contain numeric data

## Sample Data Files

The `sample/` directory contains example CSV files you can use to test:

- `sales_data.csv` - Monthly financial data with multiple numeric columns
- `product_sales.csv` - Product comparison data
- `market_share.csv` - Distribution/pie chart data

## Complete Example

Combine table and charts with custom options:

```bash
csvtotable sample/sales_data.csv financial_report.html \
  --caption "Q4 2025 Financial Report" \
  --chart line \
  --chart-x "Month" \
  --chart-y "Revenue,Expenses" \
  --chart-title "Monthly Financial Trends" \
  --chart bar \
  --chart-x "Month" \
  --chart-y "Profit" \
  --chart-title "Profit Analysis" \
  --export \
  --serve
```

This command will:
- Create an interactive table with caption
- Add a line chart showing Revenue and Expenses
- Add a bar chart showing Profit
- Enable export options
- Open the result in your browser

## Troubleshooting

**Error: "Column not found"**
- Make sure column names match exactly (case-sensitive)
- Check for extra spaces in column names

**Charts not displaying**
- Ensure Chart.js library is in templates/js/chart.min.js
- Check browser console for JavaScript errors

**Empty charts**
- Verify Y-axis columns contain numeric data
- Check that CSV has data rows (not just headers)
