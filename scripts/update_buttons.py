#!/usr/bin/env python3
"""
Script to update all "Simpan" / "Batal" buttons from OutlinedButton to ElevatedButton
with solid blocking colors (green for Simpan, red for Batal).
"""
import os
import re

BASE = "/Users/Andriyan/Documents/apx/devl/social-finance/mobile/lib"

files_to_process = [
    "features/warga/presentation/screens/special_resident_edit_screen.dart",
    "features/warga/presentation/screens/special_resident_create_screen.dart",
    "features/warga/presentation/screens/household_edit_screen.dart",
    "features/warga/presentation/screens/household_create_screen.dart",
    "features/iuran/presentation/screens/iuran_payment_screen.dart",
    "features/fund_location/presentation/screens/fund_location_screen.dart",
    "features/rw/presentation/screens/rw_fund_location_screen.dart",
]

# Pattern 1: The Container wrapper for "Simpan" buttons that looks like:
# Container(boxShadow: accent) -> SizedBox(height: 48) -> OutlinedButton
# Replace with: SizedBox(height: 48) -> ElevatedButton (solid success)

# Pattern 2: The Container wrapper for "Batal" buttons that looks like:
# Container(boxShadow: danger) -> SizedBox(height: 48) -> OutlinedButton
# Replace with: SizedBox(height: 48) -> ElevatedButton (solid danger)

simpan_pattern = r'''(                    child: Container\(
                      decoration: BoxDecoration\(
                        boxShadow: \[
                          BoxShadow\(
                            color: AppColors\.accent\.withValues\(alpha: 0\.15\),
                            blurRadius: 8,
                            offset: const Offset\(0, 0\),
                          \),
                        \],
                      \),
                      )?child: SizedBox\(
                        height: 48,
                        child: OutlinedButton\(
                          (?:key: const Key\('[^']+'\),\s+)?
                          onPressed: ([^,]+),
                          style: OutlinedButton\.styleFrom\(
                            side: const BorderSide\(color: AppColors\.accent\),
                            foregroundColor: AppColors\.accent,
                            shape: RoundedRectangleBorder\(
                              borderRadius: BorderRadius\.circular\(
                                AppRadius\.base,
                              \),
                            \),
                          \),'''

simpan_replace = r'''                  child: SizedBox(
                        height: 48,
                        child: ElevatedButton(
                          onPressed: \2,
                          style: ElevatedButton.styleFrom(
                            backgroundColor: AppColors.success,
                            foregroundColor: Colors.white,
                            shape: RoundedRectangleBorder(
                              borderRadius: BorderRadius.circular(
                                AppRadius.base,
                              ),
                            ),
                          ),'''

batal_pattern = r'''(                    child: Container\(
                      decoration: BoxDecoration\(
                        boxShadow: \[
                          BoxShadow\(
                            color: AppColors\.danger\.withValues\(alpha: 0\.15\),
                            blurRadius: 8,
                            offset: const Offset\(0, 0\),
                          \),
                        \],
                      \),
                      )?child: SizedBox\(
                        height: 48,
                        child: OutlinedButton\(
                          onPressed: ([^,]+),
                          style: OutlinedButton\.styleFrom\(
                            side: BorderSide\(color: AppColors\.danger\),
                            foregroundColor: AppColors\.danger,
                            shape: RoundedRectangleBorder\(
                              borderRadius: BorderRadius\.circular\(
                                AppRadius\.base,
                              \),
                            \),
                          \),'''

batal_replace = r'''                  child: SizedBox(
                        height: 48,
                        child: ElevatedButton(
                          onPressed: \1,
                          style: ElevatedButton.styleFrom(
                            backgroundColor: AppColors.danger,
                            foregroundColor: Colors.white,
                            shape: RoundedRectangleBorder(
                              borderRadius: BorderRadius.circular(
                                AppRadius.base,
                              ),
                            ),
                          ),'''

# For button content: update CircularProgressIndicator colors
# Simpan: AppColors.accent -> Colors.white
simpan_spinner_pattern = r'''CircularProgressIndicator\(
                                    strokeWidth: 2,
                                    backgroundColor: Colors\.transparent,
                                    valueColor: AlwaysStoppedAnimation<Color>\(
                                      AppColors\.accent,
                                    \),'''

simpan_spinner_replace = '''CircularProgressIndicator(
                                  strokeWidth: 2,
                                  backgroundColor: Colors.transparent,
                                  valueColor: AlwaysStoppedAnimation<Color>(
                                    Colors.white,
                                  ),'''

# Batal spinner doesn't exist in most places, but we need to update the border shadow removal
# The Container wrapper with boxShadow around each button needs to be removed

print("=== Starting button style updates ===")

for rel_path in files_to_process:
    full_path = os.path.join(BASE, rel_path)
    if not os.path.exists(full_path):
        print(f"  SKIP (not found): {rel_path}")
        continue
    
    with open(full_path, 'r') as f:
        content = f.read()
    
    original = content
    
    # For each file, we need to handle the button patterns
    # We'll do targeted replacements per file
    
    print(f"  Processing: {rel_path}")
    
    # Count changes
    simpan_count = len(re.findall(r'OutlinedButton', content))
    print(f"    Found {simpan_count} OutlinedButton instances")

print("=== Scan complete ===")
print("Script done. Now applying targeted edits manually per file.")
