import 'package:flutter/material.dart';

import '../../../../core/widgets/double_back_exit_scope.dart';
import '../../../../core/widgets/empty_state.dart';
import '../../../../core/widgets/menu_app_bar_title.dart';

class ReportsScreen extends StatelessWidget {
  const ReportsScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return DoubleBackExitScope(
      child: Scaffold(
        appBar: AppBar(
          centerTitle: false,
          title: const MenuAppBarTitle(title: 'Laporan'),
        ),
        body: const Center(
          child: EmptyState(
            icon: Icons.bar_chart_outlined,
            title: 'Laporan',
            description: 'Laporan keuangan akan tersedia pada tahap berikutnya.',
          ),
        ),
      ),
    );
  }
}
