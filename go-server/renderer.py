#!/usr/bin/env python3
import sys
import json
import datetime
import calendar
import math
from PIL import Image, ImageDraw, ImageFont

def wrap_text(text, font, max_width):
    """
    Wraps text into multiple lines such that no line's rendered width
    exceeds max_width. Works correctly for Devanagari and English.
    """
    words = text.split()
    lines = []
    current_line = []
    
    for word in words:
        test_line = ' '.join(current_line + [word])
        # Measure text width using font.getbbox (Pillow 10+ standard)
        bbox = font.getbbox(test_line)
        w = bbox[2] - bbox[0]
        if w <= max_width:
            current_line.append(word)
        else:
            if current_line:
                lines.append(' '.join(current_line))
                current_line = [word]
            else:
                lines.append(word)
                current_line = []
                
    if current_line:
        lines.append(' '.join(current_line))
        
    return lines if lines else [text]

def draw_weather_icon(draw, x, y, size, condition):
    """
    Draws a clean vector weather icon (outline black, accents red)
    based on the current weather condition string.
    """
    cond = condition.lower()
    cx = x + size / 2
    cy = y + size / 2
    
    if "sun" in cond or "clear" in cond:
        # Sunny
        r = size / 4
        draw.ellipse([cx - r, cy - r, cx + r, cy + r], outline=(255, 0, 0), width=2)
        # Sun rays
        ray_len = size / 6
        for i in range(8):
            angle = i * (math.pi / 4)
            x1 = cx + (r + 2) * math.cos(angle)
            y1 = cy + (r + 2) * math.sin(angle)
            x2 = cx + (r + 2 + ray_len) * math.cos(angle)
            y2 = cy + (r + 2 + ray_len) * math.sin(angle)
            draw.line([x1, y1, x2, y2], fill=(255, 0, 0), width=2)
            
    elif "cloud" in cond or "overcast" in cond or "mist" in cond or "fog" in cond:
        # Cloud outline
        r1 = size / 5
        draw.ellipse([cx - r1 - 10, cy - r1 + 5, cx - r1 + 10, cy + r1 + 5], fill=(255, 255, 255), outline=(0, 0, 0), width=2)
        draw.ellipse([cx + r1 - 10, cy - r1 + 5, cx + r1 + 10, cy + r1 + 5], fill=(255, 255, 255), outline=(0, 0, 0), width=2)
        r2 = size / 4
        draw.ellipse([cx - r2 + 2, cy - r2, cx + r2 + 2, cy + r2], fill=(255, 255, 255), outline=(0, 0, 0), width=2)
        # Clear middle section
        draw.rectangle([cx - size/3, cy + 3, cx + size/3, cy + r1 + 4], fill=(255, 255, 255))
        draw.line([cx - size/3, cy + r1 + 5, cx + size/3, cy + r1 + 5], fill=(0, 0, 0), width=2)
        
    elif "rain" in cond or "shower" in cond or "drizzle" in cond:
        # Rain Cloud
        cy_shift = cy - 6
        r1 = size / 6
        draw.ellipse([cx - r1 - 8, cy_shift - r1 + 4, cx - r1 + 8, cy_shift + r1 + 4], fill=(255, 255, 255), outline=(0, 0, 0), width=2)
        draw.ellipse([cx + r1 - 8, cy_shift - r1 + 4, cx + r1 + 8, cy_shift + r1 + 4], fill=(255, 255, 255), outline=(0, 0, 0), width=2)
        r2 = size / 5
        draw.ellipse([cx - r2 + 2, cy_shift - r2, cx + r2 + 2, cy_shift + r2], fill=(255, 255, 255), outline=(0, 0, 0), width=2)
        draw.rectangle([cx - size/3.5, cy_shift + 2, cx + size/3.5, cy_shift + r1 + 3], fill=(255, 255, 255))
        draw.line([cx - size/3.5, cy_shift + r1 + 4, cx + size/3.5, cy_shift + r1 + 4], fill=(0, 0, 0), width=2)
        # Drops
        drop_y = cy_shift + r1 + 8
        for dx in [-10, 0, 10]:
            draw.line([cx + dx - 2, drop_y, cx + dx + 1, drop_y + 6], fill=(255, 0, 0), width=2)
            
    elif "thunder" in cond or "storm" in cond:
        # Thunderstorm
        cy_shift = cy - 6
        r1 = size / 6
        draw.ellipse([cx - r1 - 8, cy_shift - r1 + 4, cx - r1 + 8, cy_shift + r1 + 4], fill=(255, 255, 255), outline=(0, 0, 0), width=2)
        draw.ellipse([cx + r1 - 8, cy_shift - r1 + 4, cx + r1 + 8, cy_shift + r1 + 4], fill=(255, 255, 255), outline=(0, 0, 0), width=2)
        r2 = size / 5
        draw.ellipse([cx - r2 + 2, cy_shift - r2, cx + r2 + 2, cy_shift + r2], fill=(255, 255, 255), outline=(0, 0, 0), width=2)
        draw.rectangle([cx - size/3.5, cy_shift + 2, cx + size/3.5, cy_shift + r1 + 3], fill=(255, 255, 255))
        draw.line([cx - size/3.5, cy_shift + r1 + 4, cx + size/3.5, cy_shift + r1 + 4], fill=(0, 0, 0), width=2)
        # Red zigzag lightning
        lightning = [
            (cx - 2, cy_shift + r1 + 5),
            (cx - 8, cy_shift + r1 + 12),
            (cx, cy_shift + r1 + 12),
            (cx - 4, cy_shift + r1 + 20)
        ]
        draw.line(lightning, fill=(255, 0, 0), width=2)
        
    elif "snow" in cond or "ice" in cond or "freeze" in cond:
        # Snow cloud
        cy_shift = cy - 6
        r1 = size / 6
        draw.ellipse([cx - r1 - 8, cy_shift - r1 + 4, cx - r1 + 8, cy_shift + r1 + 4], fill=(255, 255, 255), outline=(0, 0, 0), width=2)
        draw.ellipse([cx + r1 - 8, cy_shift - r1 + 4, cx + r1 + 8, cy_shift + r1 + 4], fill=(255, 255, 255), outline=(0, 0, 0), width=2)
        r2 = size / 5
        draw.ellipse([cx - r2 + 2, cy_shift - r2, cx + r2 + 2, cy_shift + r2], fill=(255, 255, 255), outline=(0, 0, 0), width=2)
        draw.rectangle([cx - size/3.5, cy_shift + 2, cx + size/3.5, cy_shift + r1 + 3], fill=(255, 255, 255))
        draw.line([cx - size/3.5, cy_shift + r1 + 4, cx + size/3.5, cy_shift + r1 + 4], fill=(0, 0, 0), width=2)
        # Snow dots
        drop_y = cy_shift + r1 + 8
        for dx in [-10, 0, 10]:
            draw.ellipse([cx + dx - 2, drop_y - 2, cx + dx + 2, drop_y + 2], fill=(255, 0, 0))
            
    else:
        # Partly Cloudy (Default fallback)
        r_sun = size / 5
        draw.ellipse([cx - r_sun + 8, cy - r_sun - 8, cx + r_sun + 8, cy + r_sun - 8], outline=(255, 0, 0), width=2)
        
        r1 = size / 6
        draw.ellipse([cx - r1 - 8, cy - r1 + 4, cx - r1 + 8, cy + r1 + 4], fill=(255, 255, 255), outline=(0, 0, 0), width=2)
        draw.ellipse([cx + r1 - 8, cy - r1 + 4, cx + r1 + 8, cy + r1 + 4], fill=(255, 255, 255), outline=(0, 0, 0), width=2)
        r2 = size / 5
        draw.ellipse([cx - r2 + 2, cy - r2, cx + r2 + 2, cy + r2], fill=(255, 255, 255), outline=(0, 0, 0), width=2)
        draw.rectangle([cx - size/3.5, cy + 2, cx + size/3.5, cy + r1 + 3], fill=(255, 255, 255))
        draw.line([cx - size/3.5, cy + r1 + 4, cx + size/3.5, cy + r1 + 4], fill=(0, 0, 0), width=2)

def main():
    try:
        raw_data = sys.stdin.read()
        if not raw_data:
            print("Error: Empty JSON payload on stdin", file=sys.stderr)
            sys.exit(1)
            
        data = json.loads(raw_data)
        
        width = data.get("width", 800)
        height = data.get("height", 480)
        output_path = data.get("output_path", "output.png")
        
        # Fonts
        regular_font_path = data.get("regular_font", "./assets/fonts/Mukta-Medium.ttf")
        bold_font_path = data.get("bold_font", "./assets/fonts/Mukta-Bold.ttf")
        
        try:
            font_title = ImageFont.truetype(bold_font_path, 26)
            font_header = ImageFont.truetype(bold_font_path, 20)
            font_weekday = ImageFont.truetype(bold_font_path, 14)
            font_body_bold = ImageFont.truetype(bold_font_path, 15)
            font_body_reg = ImageFont.truetype(regular_font_path, 14)
            font_body_small = ImageFont.truetype(regular_font_path, 13)
        except IOError as e:
            print(f"Error loading fonts: {e}", file=sys.stderr)
            sys.exit(1)
            
        img = Image.new('RGB', (width, height), (255, 255, 255))
        draw = ImageDraw.Draw(img)
        
        # Outer Border
        draw.rectangle([10, 10, width - 10, height - 10], outline=(0, 0, 0), width=2)
        
        # UI Selection and Visibility Settings
        layout_style = data.get("layout_style", "default")
        show_calendar = data.get("show_calendar", True)
        show_schedule = data.get("show_schedule", True)
        show_inbox = data.get("show_inbox", True)
        show_notes = data.get("show_notes", True)
        show_weather = data.get("show_weather", True)
        show_sensors = data.get("show_sensors", True)
        
        # Retrieve and normalize Weather Data (Inject Mock Data if completely empty)
        weather_data = data.get("weather")
        if not weather_data:
            weather_data = {
                "temp": 19.0,
                "condition": "Cloudy",
                "humidity": 89,
                "pressure": 29.5,
                "wind_speed": 7.39,
                "forecast": [
                    {"time": "1 PM", "temp": 18.0, "condition": "Cloudy"},
                    {"time": "2 PM", "temp": 19.0, "condition": "Cloudy"},
                    {"time": "3 PM", "temp": 20.0, "condition": "Rainy"},
                    {"time": "4 PM", "temp": 20.0, "condition": "Rainy"}
                ]
            }

        # -----------------------------------------------------------------
        # LAYOUT 1: DEFAULT (4-Quadrant layout with dynamic expanding)
        # -----------------------------------------------------------------
        if layout_style == "default":
            # Decide divider columns
            show_left = show_calendar or show_schedule
            show_right = show_inbox or show_notes
            
            if show_left and show_right:
                divider_x = int(width * 0.40)
                draw.line([divider_x, 20, divider_x, height - 20], fill=(0, 0, 0), width=1)
            elif show_left:
                divider_x = width - 10
            else:
                divider_x = 10
                
            divider_y = int(height * 0.50)
            
            # Left Column Rendering
            if show_left:
                left_col_width = divider_x - 20
                if show_calendar:
                    now = datetime.datetime.now()
                    year = now.year
                    month = now.month
                    today = now.day
                    
                    month_str = now.strftime("%B")
                    calendar_center_x = 20 + left_col_width / 2
                    draw.text((calendar_center_x, 45), f"{month_str} {year}", fill=(0, 0, 0), font=font_title, anchor="mm")
                    
                    weekdays = ["S", "M", "T", "W", "T", "F", "S"]
                    cell_width = left_col_width / 7
                    calendar_top = 75
                    for i, wd in enumerate(weekdays):
                        cx = 20 + i * cell_width + cell_width/2
                        draw.text((cx, calendar_top), wd, fill=(255, 0, 0), font=font_weekday, anchor="mm")
                        
                    first_weekday, num_days = calendar.monthrange(year, month)
                    start_weekday = (first_weekday + 1) % 7
                    
                    row_height = 25 if height < 400 else 28
                    for d in range(1, num_days + 1):
                        cell_idx = d - 1 + start_weekday
                        col = cell_idx % 7
                        row = cell_idx // 7
                        
                        cx = 20 + col * cell_width + cell_width/2
                        cy = calendar_top + 25 + row * row_height
                        
                        if d == today:
                            radius = 13
                            draw.ellipse([cx - radius, cy - radius, cx + radius, cy + radius], fill=(255, 0, 0))
                            draw.text((cx, cy - 1), str(d), fill=(255, 255, 255), font=font_body_bold, anchor="mm")
                        else:
                            draw.text((cx, cy), str(d), fill=(0, 0, 0), font=font_body_reg, anchor="mm")
                            
                # Schedule Rendering
                if show_schedule:
                    sched_start_y = divider_y + 15 if show_calendar else 25
                    sched_height = height - sched_start_y - 20
                    
                    draw.text((20, sched_start_y), "SCHEDULE", fill=(255, 0, 0), font=font_header)
                    draw.line([20, sched_start_y + 24, divider_x - 10, sched_start_y + 24], fill=(0, 0, 0), width=1)
                    
                    events = data.get("calendar", [])
                    current_y = sched_start_y + 32
                    event_spacing = 42
                    max_events = int(sched_height / event_spacing) - 1
                    if max_events < 1:
                        max_events = 1
                        
                    for i, ev in enumerate(events[:max_events]):
                        if current_y > height - 30:
                            break
                        title = ev.get("title", "")
                        time_str = ev.get("time", "")
                        
                        draw.text((25, current_y), time_str, fill=(255, 0, 0), font=font_body_bold)
                        wrapped_title_lines = wrap_text(title, font_body_small, left_col_width - 15)
                        title_line = wrapped_title_lines[0]
                        if len(wrapped_title_lines) > 1:
                            title_line += "..."
                        draw.text((25, current_y + 20), title_line, fill=(0, 0, 0), font=font_body_small)
                        current_y += event_spacing
                        
                    if not events:
                        draw.text((25, current_y), "No events today.", fill=(0, 0, 0), font=font_body_reg)
                        
            # Right Column Rendering
            if show_right:
                right_col_x = divider_x + 20
                right_col_width = width - right_col_x - 20
                
                # Inbox rendering
                if show_inbox:
                    inb_start_y = 25
                    inb_height = divider_y - inb_start_y if show_notes else height - inb_start_y - 20
                    
                    draw.text((right_col_x, inb_start_y), "INBOX", fill=(255, 0, 0), font=font_header)
                    if show_notes:
                        draw.line([right_col_x - 10, divider_y, width - 20, divider_y], fill=(0, 0, 0), width=1)
                        
                    emails = data.get("emails", [])
                    current_y = inb_start_y + 28
                    email_spacing = 50
                    max_emails = int(inb_height / email_spacing)
                    if max_emails < 1:
                        max_emails = 1
                        
                    for i, email in enumerate(emails[:max_emails]):
                        sender = email.get("sender", "")
                        subject = email.get("subject", "")
                        
                        wrapped_sender = wrap_text(sender, font_body_bold, right_col_width - 20)
                        sender_line = wrapped_sender[0]
                        if len(wrapped_sender) > 1:
                            sender_line += "..."
                        draw.text((right_col_x + 10, current_y), sender_line, fill=(0, 0, 0), font=font_body_bold)
                        
                        wrapped_subject = wrap_text(subject, font_body_small, right_col_width - 20)
                        subject_line = wrapped_subject[0]
                        if len(wrapped_subject) > 1:
                            subject_line += "..."
                        draw.text((right_col_x + 10, current_y + 21), subject_line, fill=(0, 0, 0), font=font_body_small)
                        current_y += email_spacing
                        
                    if not emails:
                        draw.text((right_col_x + 10, current_y), "No emails.", fill=(0, 0, 0), font=font_body_reg)
                        
                # Notes rendering
                if show_notes:
                    notes_start_y = divider_y + 15 if show_inbox else 25
                    draw.text((right_col_x, notes_start_y), "NOTES", fill=(255, 0, 0), font=font_header)
                    
                    notes = data.get("notes", [])
                    current_y = notes_start_y + 28
                    note_spacing = 8
                    bullet_radius = 2.5
                    
                    for note in notes:
                        if current_y > height - 30:
                            break
                        wrapped_lines = wrap_text(note, font_body_reg, right_col_width - 30)
                        bullet_y = current_y + 7
                        draw.ellipse([right_col_x + 12 - bullet_radius, bullet_y - bullet_radius,
                                      right_col_x + 12 + bullet_radius, bullet_y + bullet_radius], fill=(255, 0, 0))
                        for line in wrapped_lines:
                            if current_y > height - 30:
                                break
                            draw.text((right_col_x + 25, current_y), line, fill=(0, 0, 0), font=font_body_reg)
                            current_y += 18
                        current_y += note_spacing
                        
                    if not notes:
                        draw.text((right_col_x + 10, current_y), "No notes.", fill=(0, 0, 0), font=font_body_reg)

        # -----------------------------------------------------------------
        # LAYOUT 2: WEATHER & TASKS (Image 1 style)
        # -----------------------------------------------------------------
        elif layout_style == "weather_tasks":
            divider_x = int(width * 0.42)
            draw.line([divider_x, 20, divider_x, height - 20], fill=(0, 0, 0), width=1)
            
            # Left side: Weather (Condition, Icon, Temp, Forecast)
            if show_weather:
                temp = weather_data.get("temp", 20.0)
                cond = weather_data.get("condition", "Cloudy")
                
                # Title header
                draw.text((20, 25), "WEATHER", fill=(255, 0, 0), font=font_header)
                draw.line([20, 49, divider_x - 10, 49], fill=(0, 0, 0), width=1)
                
                # Big icon and big temperature side by side
                draw_weather_icon(draw, 25, 65, 80, cond)
                draw.text((120, 75), f"{int(temp)}°C", fill=(0, 0, 0), font=ImageFont.truetype(bold_font_path, 40))
                draw.text((120, 115), cond, fill=(0, 0, 0), font=font_body_bold)
                
                # Divider for Forecast
                draw.line([20, 160, divider_x - 10, 160], fill=(0, 0, 0), width=1)
                draw.text((20, 170), "FORECAST", fill=(255, 0, 0), font=font_body_bold)
                
                # Draw hourly forecast items vertically
                forecasts = weather_data.get("forecast", [])
                current_y = 195
                for fc in forecasts[:4]:
                    ftime = fc.get("time", "")
                    ftemp = fc.get("temp", 20.0)
                    fcond = fc.get("condition", "Cloudy")
                    
                    # draw time, icon, and temp on a single row
                    draw.text((25, current_y), ftime, fill=(0, 0, 0), font=font_body_reg)
                    draw_weather_icon(draw, 100, current_y - 8, 30, fcond)
                    draw.text((160, current_y), f"{int(ftemp)}°C", fill=(255, 0, 0), font=font_body_bold)
                    current_y += 38
            
            # Right side: Tasks (Inbox) + Next Calendar Event
            right_col_x = divider_x + 20
            right_col_width = width - right_col_x - 20
            
            # List of tasks
            draw.text((right_col_x, 25), "WORK", fill=(255, 0, 0), font=font_header)
            draw.line([right_col_x - 10, 49, width - 20, 49], fill=(0, 0, 0), width=1)
            
            emails = data.get("emails", [])
            current_y = 65
            for email in emails[:5]:
                subject = email.get("subject", "")
                wrapped_subject = wrap_text(subject, font_body_reg, right_col_width - 15)
                subject_line = wrapped_subject[0]
                if len(wrapped_subject) > 1:
                    subject_line += "..."
                
                draw.text((right_col_x + 10, current_y), f"• {subject_line}", fill=(0, 0, 0), font=font_body_reg)
                current_y += 30
                
            # Next call at the bottom
            draw.line([right_col_x - 10, height - 60, width - 20, height - 60], fill=(0, 0, 0), width=1)
            events = data.get("calendar", [])
            next_call_str = "Next call: None scheduled"
            if events:
                next_call_str = f"Next event: {events[0].get('title', '')} at {events[0].get('time', '')}"
            
            # Truncate next call text
            wrapped_call = wrap_text(next_call_str, font_body_bold, right_col_width - 15)
            call_line = wrapped_call[0]
            if len(wrapped_call) > 1:
                call_line += "..."
            draw.text((right_col_x + 10, height - 42), call_line, fill=(0, 0, 0), font=font_body_bold)

        # -----------------------------------------------------------------
        # LAYOUT 3: WEATHER & CALENDAR (Image 2 style)
        # -----------------------------------------------------------------
        elif layout_style == "weather_calendar":
            divider_x = int(width * 0.42)
            draw.line([divider_x, 20, divider_x, height - 20], fill=(0, 0, 0), width=1)
            
            # Left side: Current Weather info + 4 Sensor cards
            if show_weather:
                temp = weather_data.get("temp", 20.0)
                cond = weather_data.get("condition", "Cloudy")
                
                # Draw weather header
                now = datetime.datetime.now()
                draw.text((20, 25), now.strftime("%A"), fill=(0, 0, 0), font=font_header)
                draw.text((20, 48), now.strftime("%B %d"), fill=(0, 0, 0), font=font_body_bold)
                
                draw_weather_icon(draw, 140, 20, 50, cond)
                
                # 4 grids for Sensors
                grid_y = 90
                grid_w = (divider_x - 30) / 2
                grid_h = 75
                
                sensors = [
                    {"label": "Temperature", "value": f"{int(temp)}°C", "icon": "temp"},
                    {"label": "Humidity", "value": f"{weather_data.get('humidity', 70)}%", "icon": "humidity"},
                    {"label": "Air Pressure", "value": f"{weather_data.get('pressure', 1013)} hPa", "icon": "pressure"},
                    {"label": "Wind Speed", "value": f"{weather_data.get('wind_speed', 5.0)} m/s", "icon": "wind"}
                ]
                
                for idx, s in enumerate(sensors):
                    col = idx % 2
                    row = idx // 2
                    
                    gx = 18 + col * (grid_w + 8)
                    gy = grid_y + row * (grid_h + 8)
                    
                    # Draw a nice clean rounded card border
                    draw.rectangle([gx, gy, gx + grid_w, gy + grid_h], outline=(0, 0, 0), width=1)
                    
                    # Labels & Value
                    draw.text((gx + 6, gy + 6), s["label"], fill=(0, 0, 0), font=font_body_small)
                    draw.text((gx + 6, gy + 26), s["value"], fill=(255, 0, 0), font=font_body_bold)
            
            # Right side: Calendar list
            right_col_x = divider_x + 20
            right_col_width = width - right_col_x - 20
            
            draw.text((right_col_x, 25), "Calendar", fill=(255, 0, 0), font=font_header)
            draw.line([right_col_x - 10, 49, width - 20, 49], fill=(0, 0, 0), width=1)
            
            events = data.get("calendar", [])
            current_y = 65
            for ev in events[:6]:
                if current_y > height - 40:
                    break
                title = ev.get("title", "")
                time_str = ev.get("time", "")
                
                draw.text((right_col_x + 10, current_y), time_str, fill=(255, 0, 0), font=font_body_bold)
                wrapped_title = wrap_text(title, font_body_reg, right_col_width - 20)
                draw.text((right_col_x + 10, current_y + 18), wrapped_title[0], fill=(0, 0, 0), font=font_body_reg)
                current_y += 45

        # -----------------------------------------------------------------
        # LAYOUT 4: WEATHER 7-DAY FORECAST (Image 3 style)
        # -----------------------------------------------------------------
        elif layout_style == "weather_forecast":
            divider_x = int(width * 0.38)
            draw.line([divider_x, 20, divider_x, height - 20], fill=(0, 0, 0), width=1)
            
            # Draw beautiful native dithered checkerboard shading in the left column
            for py in range(12, height - 12, 2):
                for px in range(12, divider_x - 2, 2):
                    offset = 0 if (py // 2) % 2 == 0 else 1
                    draw.point((px + offset, py), fill=(0, 0, 0))
                    
            # Clear a panel on top of dither for Weather Info
            draw.rectangle([20, 20, divider_x - 10, height - 20], fill=(255, 255, 255), outline=(0, 0, 0), width=2)
            
            # Render Weather left side highlights
            if show_weather:
                temp = weather_data.get("temp", 20.0)
                cond = weather_data.get("condition", "Cloudy")
                
                now = datetime.datetime.now()
                draw.text((30, 35), now.strftime("%b %d %A"), fill=(0, 0, 0), font=font_body_bold)
                draw.text((30, 55), cond, fill=(0, 0, 0), font=font_body_reg)
                
                draw_weather_icon(draw, 30, 80, 90, cond)
                
                draw.text((30, 185), f"{temp:.1f}°", fill=(0, 0, 0), font=ImageFont.truetype(bold_font_path, 46))
                
                # Thermometer / Humidity stats
                draw.text((30, 245), f"Humidity: {weather_data.get('humidity', 70)}%", fill=(0, 0, 0), font=font_body_reg)
                draw.text((30, 268), f"Wind: {weather_data.get('wind_speed', 5.0)} m/s", fill=(0, 0, 0), font=font_body_reg)
            
            # Right column: 7-Day weather forecast table
            right_col_x = divider_x + 20
            right_col_width = width - right_col_x - 20
            
            weather_city = data.get("weather_city", "New Delhi,IN")
            draw.text((right_col_x, 25), f"📍 {weather_city}", fill=(0, 0, 0), font=font_header)
            draw.line([right_col_x - 10, 49, width - 20, 49], fill=(0, 0, 0), width=1)
            
            # Weather forecasts table list (5 days)
            forecasts = weather_data.get("forecast", [])
            current_y = 60
            
            # Generate simulated days list (using actual weekday names)
            now = datetime.datetime.now()
            for idx in range(5):
                day_time = now + datetime.timedelta(days=idx)
                day_name = day_time.strftime("%a")
                if idx == 0:
                    day_name = "Today"
                
                # Grab mock/real temps for forecast rows
                fc_temp = temp
                fc_cond = cond
                if idx < len(forecasts):
                    fc_temp = forecasts[idx].get("temp", temp)
                    fc_cond = forecasts[idx].get("condition", cond)
                    
                draw.text((right_col_x + 10, current_y + 10), day_name, fill=(0, 0, 0), font=font_body_bold)
                draw_weather_icon(draw, right_col_x + 100, current_y, 30, fc_cond)
                draw.text((right_col_x + 160, current_y + 10), fc_cond, fill=(0, 0, 0), font=font_body_small)
                
                temp_range_str = f"{int(fc_temp - 2)}° - {int(fc_temp + 3)}°"
                draw.text((width - 80, current_y + 10), temp_range_str, fill=(255, 0, 0), font=font_body_bold)
                
                # Divider between rows
                draw.line([right_col_x + 5, current_y + 36, width - 20, current_y + 36], fill=(0, 0, 0), width=1)
                current_y += 38

        # -----------------------------------------------------------------
        # LAYOUT 5: CLOCK & SENSORS (Image 4 style)
        # -----------------------------------------------------------------
        elif layout_style == "clock_sensors":
            # Header Date and Big Digital Clock
            now = datetime.datetime.now()
            draw.text((width / 2, 25), now.strftime("%A, %B %d, %Y"), fill=(0, 0, 0), font=font_body_bold, anchor="mm")
            
            # Clock centered
            clock_str = now.strftime("%I:%M %p")
            draw.text((width / 2, 65), clock_str, fill=(0, 0, 0), font=ImageFont.truetype(bold_font_path, 42), anchor="mm")
            
            # Horizontal divider line under clock
            draw.line([20, 100, width - 20, 100], fill=(0, 0, 0), width=1)
            
            # 3-column split below clock line (y: 100 to height - 20)
            col_w = (width - 40) / 3
            y_start = 110
            
            # Left column: Smart home sensors list
            if show_sensors:
                draw.text((25, y_start), "SENSORS", fill=(255, 0, 0), font=font_body_bold)
                draw.line([25, y_start + 18, col_w, y_start + 18], fill=(0, 0, 0), width=1)
                
                sensor_y = y_start + 26
                sensor_items = [
                    "Front door: Closed",
                    "Rear door: Closed",
                    f"Yard Temp: {weather_data.get('temp', 20.0)}°C",
                    f"Humidity: {weather_data.get('humidity', 70)}%",
                    f"Soil Moisture: 30%",
                    f"Pressure: {weather_data.get('pressure', 1013.0)} hPa"
                ]
                for item in sensor_items:
                    draw.text((25, sensor_y), f"• {item}", fill=(0, 0, 0), font=font_body_small)
                    sensor_y += 24
                    
            # Vertical split lines
            draw.line([col_w + 10, y_start, col_w + 10, height - 20], fill=(0, 0, 0), width=1)
            draw.line([2 * col_w + 10, y_start, 2 * col_w + 10, height - 20], fill=(0, 0, 0), width=1)
            
            # Middle column: Current Weather Highlight
            if show_weather:
                temp = weather_data.get("temp", 20.0)
                cond = weather_data.get("condition", "Cloudy")
                
                mid_x = col_w + 20
                draw.text((mid_x + col_w/2 - 10, y_start), "CURRENTLY", fill=(255, 0, 0), font=font_body_bold, anchor="mm")
                draw.line([mid_x, y_start + 18, mid_x + col_w - 20, y_start + 18], fill=(0, 0, 0), width=1)
                
                draw_weather_icon(draw, mid_x + col_w/4, y_start + 35, 60, cond)
                draw.text((mid_x + col_w/2 - 10, y_start + 120), f"{int(temp)}°C", fill=(0, 0, 0), font=ImageFont.truetype(bold_font_path, 32), anchor="mm")
                draw.text((mid_x + col_w/2 - 10, y_start + 155), cond, fill=(0, 0, 0), font=font_body_bold, anchor="mm")
                
            # Right column: Hourly / Daily forecasts boxes
            right_x = 2 * col_w + 20
            right_w = width - right_x - 20
            
            draw.text((right_x, y_start), "FORECAST", fill=(255, 0, 0), font=font_body_bold)
            draw.line([right_x, y_start + 18, width - 25, y_start + 18], fill=(0, 0, 0), width=1)
            
            forecasts = weather_data.get("forecast", [])
            fc_y = y_start + 26
            for idx in range(min(5, len(forecasts))):
                fc = forecasts[idx]
                ftime = fc.get("time", "")
                ftemp = fc.get("temp", 20.0)
                fcond = fc.get("condition", "Cloudy")
                
                draw.text((right_x, fc_y), ftime, fill=(0, 0, 0), font=font_body_small)
                draw_weather_icon(draw, right_x + 50, fc_y - 4, 20, fcond)
                draw.text((right_x + 85, fc_y), f"{int(ftemp)}°C", fill=(255, 0, 0), font=font_body_bold)
                fc_y += 26
                
        # Save output image
        img.save(output_path, "PNG")
        
    except Exception as e:
        print(f"Exception inside Python renderer: {e}", file=sys.stderr)
        sys.exit(1)

if __name__ == "__main__":
    main()
