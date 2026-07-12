#!/usr/bin/env python3
import sys
import json
import datetime
import calendar
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

def main():
    try:
        # Load layout data from standard input
        raw_data = sys.stdin.read()
        if not raw_data:
            print("Error: Empty JSON payload on stdin", file=sys.stderr)
            sys.exit(1)
            
        data = json.loads(raw_data)
        
        # Dimensions
        width = data.get("width", 800)
        height = data.get("height", 480)
        output_path = data.get("output_path", "output.png")
        
        # Fonts
        regular_font_path = data.get("regular_font", "./assets/fonts/Poppins-Regular.ttf")
        bold_font_path = data.get("bold_font", "./assets/fonts/Poppins-Bold.ttf")
        
        # Setup fonts with sizes
        try:
            font_title = ImageFont.truetype(bold_font_path, 22)
            font_header = ImageFont.truetype(bold_font_path, 16)
            font_weekday = ImageFont.truetype(bold_font_path, 12)
            font_body_bold = ImageFont.truetype(bold_font_path, 12)
            font_body_reg = ImageFont.truetype(regular_font_path, 12)
            font_body_small = ImageFont.truetype(regular_font_path, 11)
            font_status = ImageFont.truetype(regular_font_path, 9)
        except IOError as e:
            print(f"Error loading fonts: {e}", file=sys.stderr)
            sys.exit(1)
            
        # Create image
        # Standard colors: White (255, 255, 255), Black (0, 0, 0), Red (255, 0, 0)
        img = Image.new('RGB', (width, height), (255, 255, 255))
        draw = ImageDraw.Draw(img)
        
        # 1. Outer Border
        draw.rectangle([10, 10, width - 10, height - 10], outline=(0, 0, 0), width=2)
        
        # 2. Vertical Divider
        divider_x = 320
        draw.line([divider_x, 20, divider_x, height - 20], fill=(0, 0, 0), width=1)
        
        # 3. Calendar Column (Left Column)
        now = datetime.datetime.now()
        year = now.year
        month = now.month
        today = now.day
        
        # Month Header (e.g. July 2026)
        month_str = now.strftime("%B")
        draw.text((20 + 280/2, 45), f"{month_str} {year}", fill=(0, 0, 0), font=font_title, anchor="mm")
        
        # Weekday labels
        weekdays = ["S", "M", "T", "W", "T", "F", "S"]
        cell_width = 280 / 7
        calendar_top = 75
        for i, wd in enumerate(weekdays):
            cx = 20 + i * cell_width + cell_width/2
            draw.text((cx, calendar_top), wd, fill=(255, 0, 0), font=font_weekday, anchor="mm")
            
        # Day numbers
        first_weekday, num_days = calendar.monthrange(year, month)
        # Shift weekday to Sunday start: Sunday=0, Monday=1... Saturday=6
        start_weekday = (first_weekday + 1) % 7
        
        row_height = 26
        for d in range(1, num_days + 1):
            cell_idx = d - 1 + start_weekday
            col = cell_idx % 7
            row = cell_idx // 7
            
            cx = 20 + col * cell_width + cell_width/2
            cy = calendar_top + 25 + row * row_height
            
            if d == today:
                # Highlight today's date in red
                radius = 11
                draw.ellipse([cx - radius, cy - radius, cx + radius, cy + radius], fill=(255, 0, 0))
                draw.text((cx, cy - 1), str(d), fill=(255, 255, 255), font=font_body_bold, anchor="mm")
            else:
                draw.text((cx, cy), str(d), fill=(0, 0, 0), font=font_body_reg, anchor="mm")
                
        # 4. Schedule Section (Bottom Left, below calendar)
        schedule_y = 265
        draw.text((20, schedule_y), "SCHEDULE", fill=(255, 0, 0), font=font_header)
        
        # Draw horizontal sub-divider in left column
        draw.line([20, schedule_y + 22, divider_x - 10, schedule_y + 22], fill=(0, 0, 0), width=1)
        
        events = data.get("calendar", [])
        current_y = schedule_y + 30
        max_events = 3
        event_spacing = 38
        
        for i, ev in enumerate(events[:max_events]):
            title = ev.get("title", "")
            time_str = ev.get("time", "")
            
            # Format time: e.g. "09:00 - 10:00" (Bold Red/Black)
            draw.text((25, current_y), time_str, fill=(255, 0, 0), font=font_body_bold)
            
            # Format Title (Devanagari supported Poppins Regular)
            wrapped_title_lines = wrap_text(title, font_body_small, 260)
            draw.text((25, current_y + 14), wrapped_title_lines[0], fill=(0, 0, 0), font=font_body_small)
            
            current_y += event_spacing
            
        if not events:
            draw.text((25, current_y), "No events today.", fill=(128, 128, 128), font=font_body_reg)
            
        # Draw generation time at the bottom left
        last_updated_str = data.get("last_updated", "")
        if not last_updated_str:
            last_updated_str = now.strftime('%Y-%m-%d %H:%M:%S')
        draw.text((30, height - 23), f"Updated: {last_updated_str}", fill=(128, 128, 128), font=font_status)
        
        # 5. Right Column Dividers
        right_col_x = 340
        right_col_width = width - right_col_x - 20
        
        # Horizontal divider in right column
        divider_y = 240
        draw.line([right_col_x - 10, divider_y, width - 20, divider_y], fill=(0, 0, 0), width=1)
        
        # 6. Emails (Top Right)
        emails_y = 25
        draw.text((right_col_x, emails_y), "INBOX", fill=(255, 0, 0), font=font_header)
        
        emails = data.get("emails", [])
        current_y = emails_y + 25
        email_spacing = 40
        max_emails = 4
        
        for i, email in enumerate(emails[:max_emails]):
            sender = email.get("sender", "")
            subject = email.get("subject", "")
            
            # Sender (Bold Black)
            # Truncate if too long
            if len(sender) > 40:
                sender = sender[:37] + "..."
            draw.text((right_col_x + 10, current_y), sender, fill=(0, 0, 0), font=font_body_bold)
            
            # Subject (Regular Grayish/Black)
            if len(subject) > 65:
                subject = subject[:62] + "..."
            draw.text((right_col_x + 10, current_y + 15), subject, fill=(80, 80, 80), font=font_body_small)
            
            current_y += email_spacing
            
        if not emails:
            draw.text((right_col_x + 10, current_y), "No emails.", fill=(128, 128, 128), font=font_body_reg)
            
        # 7. Notes (Bottom Right)
        notes_y = 255
        draw.text((right_col_x, notes_y), "NOTES", fill=(255, 0, 0), font=font_header)
        
        notes = data.get("notes", [])
        current_y = notes_y + 28
        note_spacing = 8
        bullet_radius = 2.5
        
        for note in notes:
            if current_y > height - 30:
                break
                
            # Wrap text to column width
            wrapped_lines = wrap_text(note, font_body_reg, right_col_width - 30)
            
            # Draw red circular bullet
            bullet_y = current_y + 7
            draw.ellipse([right_col_x + 12 - bullet_radius, bullet_y - bullet_radius,
                          right_col_x + 12 + bullet_radius, bullet_y + bullet_radius], fill=(255, 0, 0))
                          
            # Draw text lines
            for line in wrapped_lines:
                if current_y > height - 30:
                    break
                draw.text((right_col_x + 25, current_y), line, fill=(0, 0, 0), font=font_body_reg)
                current_y += 16
                
            current_y += note_spacing
            
        if not notes:
            draw.text((right_col_x + 10, current_y), "No notes.", fill=(128, 128, 128), font=font_body_reg)
            
        # Save output image
        img.save(output_path, "PNG")
        
    except Exception as e:
        print(f"Exception inside Python renderer: {e}", file=sys.stderr)
        sys.exit(1)

if __name__ == "__main__":
    main()
