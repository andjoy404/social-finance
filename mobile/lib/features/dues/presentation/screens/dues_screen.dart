import 'package:flutter/material.dart';

import '../../../../core/widgets/double_back_exit_scope.dart';
import '../../../../core/widgets/empty_state.dart';
import '../../../../core/widgets/menu_app_bar_title.dart';

class DuesScreen extends StatelessWidget {
  const DuesScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return DoubleBackExitScope(
      child: Scaffold(
        appBar: AppBar(
          centerTitle: false,
          title: const MenuAppBarTitle(title: 'Iuran'),
        ),
        body: const Center(
          child: EmptyState(
            icon: Icons.receipt_long_outlined,
            title: 'Iuran',
            description:
                'Pengelolaan iuran warga akan tersedia pada tahap berikutnya.',
          ),
        ),
      ),
    );
  }
}
