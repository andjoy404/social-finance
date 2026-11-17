const fs = require('fs');
let content = fs.readFileSync('security_absen_screen.dart', 'utf8');

// 1. Wrap TabBarView children dengan Expanded
content = content.replace(
  /TabBarView\(\s+controller: _tabController,\s+children: \[\s+_buildAbsenAndScheduleTab\(currentUserId\),\s+_buildHistoryAndCorrectionTab\(\),\s+\],\s+\)/,
  `TabBarView(
              controller: _tabController,
              children: [
                Expanded(child: _buildAbsenAndScheduleTab(currentUserId)),
                Expanded(child: _buildHistoryAndCorrectionTab()),
              ],
            )`
);

// 2. _buildAbsenAndScheduleTab: Hapus RefreshIndicator wrapper
content = content.replace(
  /return RefreshIndicator\(\s+onRefresh: _loadData,\s+child: ListView\(/,
  'return ListView('
);

// 3. _buildHistoryAndCorrectionTab: Hapus RefreshIndicator wrapper
// Find the second occurrence of the pattern
const parts = content.split('return RefreshIndicator(\n      onRefresh: _loadData,\n      child: ListView(');
if (parts.length > 1) {
  content = 'return RefreshIndicator(\n      onRefresh: _loadData,\n      child: ListView(' + parts[1] + 
             parts.slice(1).join('return RefreshIndicator(\n      onRefresh: _loadData,\n      child: ListView(');
}

// Actually let's just replace the second occurrence directly
// Split by the pattern, replace second one, join back
const parts2 = content.match(/Widget _buildHistoryAndCorrectionTab[\s\S]*?return RefreshIndicator\(\s+onRefresh: _loadData,\s+child: ListView\(/);
if (parts2) {
  console.log('Found _buildHistoryAndCorrectionTab pattern');
}

fs.writeFileSync('security_absen_screen.dart', content, 'utf8');
console.log('File updated');
