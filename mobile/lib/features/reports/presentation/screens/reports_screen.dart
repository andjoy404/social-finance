import 'package:flutter/material.dart';
import '../../../../core/widgets/empty_state.dart';

class ReportsScreen extends StatelessWidget {
  const ReportsScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Laporan')),
      body: const Center(
        child: EmptyState(
          icon: Icons.bar_chart_outlined,
          title: 'Laporan',
          description: 'Laporan keuangan akan tersedia pada tahap berikutnya.',
        ),
      ),
    );
  }
}
