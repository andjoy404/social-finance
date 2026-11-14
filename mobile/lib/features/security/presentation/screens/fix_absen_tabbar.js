const fs = require('fs');
let content = fs.readFileSync('security_absen_screen.dart', 'utf8');

// 1. Wrap TabBarView children with Expanded
content = content.replace(
  /TabBarView\(\s+controller: _tabController,\s+children: \[\s+_buildAbsenAndScheduleTab\(currentUserId\),\s+_buildHistoryAndCorrectionTab\(\),\s+\],\s+\)/,
  'TabBarView(\n              controller: _tabController,\n              children: [\n                Expanded(child: _buildAbsenAndScheduleTab(currentUserId)),\n                Expanded(child: _buildHistoryAndCorrectionTab()),\n              ],\n            )'
);

// 2. For _buildAbsenAndScheduleTab: Replace RefreshIndicator+ListView with Expanded+ListView
const absenPattern = /return RefreshIndicator\(\s+onRefresh: _loadData,\s+child: ListView\(\s+padding: const EdgeInsets\.all\(16\),\s+children: \[\s+\/\/ Today Shift Card/;
if (absenPattern.test(content)) {
  content = content.replace(
    absenPattern,
    'return ListView(\n        padding: const EdgeInsets.all(16),\n        children: [\n          // Today Shift Card'
  );
  console.log('Removed RefreshIndicator from _buildAbsenAndScheduleTab');
}

// 3. For _buildHistoryAndCorrectionTab: Replace RefreshIndicator+ListView with Expanded+ListView
const historyPattern = /return RefreshIndicator\(\s+onRefresh: _loadData,\s+child: ListView\(\s+padding: const EdgeInsets\.all\(16\),\s+children: \[\s+const Text\(\s+'Riwayat Absensi'/;
if (historyPattern.test(content)) {
  content = content.replace(
    historyPattern,
    'return ListView(\n        padding: const EdgeInsets.all(16),\n        children: [\n          const Text(\n            \'Riwayat Absensi\''
  );
  console.log('Removed RefreshIndicator from _buildHistoryAndCorrectionTab');
}

fs.writeFileSync('security_absen_screen.dart', content, 'utf8');
console.log('File updated successfully');
