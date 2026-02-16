from __future__ import unicode_literals
import json


class ChartConfig(object):
    """Configuration for a single chart"""
    
    def __init__(self, chart_type, **kwargs):
        """
        Initialize chart configuration.
        
        Args:
            chart_type: Type of chart ('bar', 'line', 'pie')
            **kwargs: Additional configuration options
        """
        self.type = chart_type
        self.x_column = kwargs.get('x_column')
        self.y_columns = kwargs.get('y_columns', [])
        self.labels_column = kwargs.get('labels_column')
        self.values_column = kwargs.get('values_column')
        self.title = kwargs.get('title', '')
        self.colors = kwargs.get('colors')
    
    def to_dict(self):
        """Convert config to dictionary"""
        return {
            'type': self.type,
            'x_column': self.x_column,
            'y_columns': self.y_columns,
            'labels_column': self.labels_column,
            'values_column': self.values_column,
            'title': self.title,
            'colors': self.colors
        }


def generate_chart_data(headers, rows, chart_config):
    """
    Extract and format data for chart from CSV rows.
    
    Args:
        headers: List of column headers
        rows: List of data rows
        chart_config: ChartConfig object
    
    Returns:
        Dictionary containing Chart.js configuration
    """
    if chart_config.type == 'pie':
        return _generate_pie_chart(headers, rows, chart_config)
    elif chart_config.type in ['bar', 'line']:
        return _generate_bar_line_chart(headers, rows, chart_config)
    else:
        raise ValueError("Unsupported chart type: {}".format(chart_config.type))


def _generate_pie_chart(headers, rows, chart_config):
    """Generate pie chart configuration"""
    if not chart_config.labels_column or not chart_config.values_column:
        raise ValueError("Pie chart requires labels_column and values_column")
    
    try:
        label_idx = headers.index(chart_config.labels_column)
        value_idx = headers.index(chart_config.values_column)
    except ValueError as e:
        raise ValueError("Column not found in CSV headers: {}".format(str(e)))
    
    labels = []
    values = []
    
    for row in rows:
        if row and len(row) > max(label_idx, value_idx):
            try:
                labels.append(row[label_idx])
                values.append(float(row[value_idx]))
            except (ValueError, TypeError):
                continue
    
    colors = chart_config.colors or generate_colors(len(labels))
    
    return {
        'type': 'pie',
        'data': {
            'labels': labels,
            'datasets': [{
                'data': values,
                'backgroundColor': colors,
                'borderWidth': 1
            }]
        },
        'options': {
            'responsive': True,
            'maintainAspectRatio': True,
            'plugins': {
                'legend': {
                    'position': 'right',
                    'labels': {
                        'padding': 10,
                        'font': {
                            'size': 12
                        }
                    }
                },
                'title': {
                    'display': bool(chart_config.title),
                    'text': chart_config.title,
                    'font': {
                        'size': 16,
                        'weight': 'bold'
                    },
                    'padding': 20
                },
                'tooltip': {
                    'callbacks': {}
                }
            }
        }
    }


def _generate_bar_line_chart(headers, rows, chart_config):
    """Generate bar or line chart configuration"""
    if not chart_config.x_column or not chart_config.y_columns:
        raise ValueError("{} chart requires x_column and y_columns".format(
            chart_config.type.capitalize()))
    
    try:
        x_idx = headers.index(chart_config.x_column)
    except ValueError:
        raise ValueError("X-axis column '{}' not found in CSV headers".format(
            chart_config.x_column))
    
    labels = []
    for row in rows:
        if row and len(row) > x_idx:
            labels.append(row[x_idx])
    
    datasets = []
    base_colors = chart_config.colors or generate_colors(len(chart_config.y_columns))
    
    for i, y_col in enumerate(chart_config.y_columns):
        try:
            y_idx = headers.index(y_col)
        except ValueError:
            raise ValueError("Y-axis column '{}' not found in CSV headers".format(y_col))
        
        data = []
        for row in rows:
            if row and len(row) > y_idx:
                try:
                    data.append(float(row[y_idx]))
                except (ValueError, TypeError):
                    data.append(0)
            else:
                data.append(0)
        
        color = base_colors[i] if i < len(base_colors) else base_colors[0]
        
        dataset = {
            'label': y_col,
            'data': data,
            'borderColor': color,
            'backgroundColor': color if chart_config.type == 'bar' else _add_transparency(color, 0.2),
            'borderWidth': 2,
        }
        
        if chart_config.type == 'line':
            dataset['fill'] = True
            dataset['tension'] = 0.4
        
        datasets.append(dataset)
    
    return {
        'type': chart_config.type,
        'data': {
            'labels': labels,
            'datasets': datasets
        },
        'options': {
            'responsive': True,
            'maintainAspectRatio': True,
            'plugins': {
                'legend': {
                    'position': 'top',
                    'labels': {
                        'padding': 15,
                        'font': {
                            'size': 12
                        }
                    }
                },
                'title': {
                    'display': bool(chart_config.title),
                    'text': chart_config.title,
                    'font': {
                        'size': 16,
                        'weight': 'bold'
                    },
                    'padding': 20
                }
            },
            'scales': {
                'y': {
                    'beginAtZero': True,
                    'grid': {
                        'display': True,
                        'color': 'rgba(0, 0, 0, 0.05)'
                    }
                },
                'x': {
                    'grid': {
                        'display': False
                    }
                }
            }
        }
    }


def generate_colors(count):
    """
    Generate a list of distinct colors for chart elements.
    
    Args:
        count: Number of colors needed
    
    Returns:
        List of color hex codes
    """
    colors = [
        '#FF6384',  # Pink
        '#36A2EB',  # Blue
        '#FFCE56',  # Yellow
        '#4BC0C0',  # Teal
        '#9966FF',  # Purple
        '#FF9F40',  # Orange
        '#FF6384',  # Pink (repeat)
        '#C9CBCF',  # Gray
        '#4BC0C0',  # Teal (repeat)
        '#FF6384'   # Pink (repeat)
    ]
    
    # Cycle through colors if we need more than available
    result = []
    for i in range(count):
        result.append(colors[i % len(colors)])
    
    return result


def _add_transparency(hex_color, alpha):
    """
    Convert hex color to rgba with transparency.
    
    Args:
        hex_color: Color in hex format (e.g., '#FF6384')
        alpha: Transparency level (0.0 to 1.0)
    
    Returns:
        Color in rgba format
    """
    hex_color = hex_color.lstrip('#')
    r = int(hex_color[0:2], 16)
    g = int(hex_color[2:4], 16)
    b = int(hex_color[4:6], 16)
    return 'rgba({}, {}, {}, {})'.format(r, g, b, alpha)


def auto_detect_charts(headers, rows, max_charts=2):
    """
    Automatically detect and suggest appropriate charts from data.
    
    Args:
        headers: List of column headers
        rows: List of data rows (sample is sufficient)
        max_charts: Maximum number of charts to auto-generate
    
    Returns:
        List of ChartConfig objects
    """
    if not headers or not rows:
        return []
    
    # Analyze first 100 rows to determine column types
    sample_size = min(100, len(rows))
    sample_rows = rows[:sample_size]
    
    numeric_cols = []
    text_cols = []
    date_like_cols = []
    
    for i, header in enumerate(headers):
        sample_values = [row[i] for row in sample_rows if row and len(row) > i and row[i]]
        
        if not sample_values:
            continue
        
        # Check if numeric
        numeric_count = 0
        for val in sample_values[:20]:  # Check first 20 non-empty values
            try:
                float(val)
                numeric_count += 1
            except (ValueError, TypeError):
                pass
        
        if numeric_count > len(sample_values[:20]) * 0.8:  # 80% numeric
            numeric_cols.append(header)
        else:
            text_cols.append(header)
            # Check if date-like
            if any(keyword in header.lower() for keyword in ['date', 'time', 'month', 'year', 'day']):
                date_like_cols.append(header)
    
    charts = []
    
    # Generate bar chart if we have categorical and numeric data
    if text_cols and numeric_cols and len(charts) < max_charts:
        # Use first text column as X-axis, first numeric as Y-axis
        charts.append(ChartConfig(
            'bar',
            x_column=text_cols[0],
            y_columns=[numeric_cols[0]],
            title='{} by {}'.format(numeric_cols[0], text_cols[0])
        ))
    
    # Generate line chart for time series data
    if date_like_cols and numeric_cols and len(charts) < max_charts:
        charts.append(ChartConfig(
            'line',
            x_column=date_like_cols[0],
            y_columns=[numeric_cols[0]],
            title='{} over {}'.format(numeric_cols[0], date_like_cols[0])
        ))
    
    # Generate pie chart if we have 2-10 categories and one numeric column
    if text_cols and numeric_cols and len(charts) < max_charts:
        if 2 <= len(sample_rows) <= 10:
            charts.append(ChartConfig(
                'pie',
                labels_column=text_cols[0],
                values_column=numeric_cols[0],
                title='{} Distribution'.format(numeric_cols[0])
            ))
    
    return charts[:max_charts]


def parse_chart_options(chart_types, chart_x, chart_y, chart_labels, 
                       chart_values, chart_titles):
    """
    Parse CLI chart options into ChartConfig objects.
    
    Args:
        chart_types: Tuple of chart types
        chart_x: Tuple of x-axis columns
        chart_y: Tuple of y-axis columns (comma-separated)
        chart_labels: Tuple of label columns for pie charts
        chart_values: Tuple of value columns for pie charts
        chart_titles: Tuple of chart titles
    
    Returns:
        List of ChartConfig objects
    """
    charts = []
    
    if not chart_types:
        return charts
    
    for i, chart_type in enumerate(chart_types):
        if chart_type == 'pie':
            labels_col = chart_labels[i] if i < len(chart_labels) else None
            values_col = chart_values[i] if i < len(chart_values) else None
            
            if not labels_col or not values_col:
                continue
            
            charts.append(ChartConfig(
                'pie',
                labels_column=labels_col,
                values_column=values_col,
                title=chart_titles[i] if i < len(chart_titles) else ''
            ))
        
        elif chart_type in ['bar', 'line']:
            x_col = chart_x[i] if i < len(chart_x) else None
            y_cols_str = chart_y[i] if i < len(chart_y) else None
            
            if not x_col or not y_cols_str:
                continue
            
            # Parse comma-separated y columns
            y_cols = [col.strip() for col in y_cols_str.split(',')]
            
            charts.append(ChartConfig(
                chart_type,
                x_column=x_col,
                y_columns=y_cols,
                title=chart_titles[i] if i < len(chart_titles) else ''
            ))
    
    return charts
