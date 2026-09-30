import 'package:flutter/material.dart';

import '../../../../core/widgets/double_back_exit_scope.dart';
import '../../../../core/widgets/empty_state.dart';
import '../../../../core/widgets/menu_app_bar_title.dart';

class CashScreen extends StatelessWidget {
  const CashScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return DoubleBackExitScope(
      child: Scaffold(
        appBar: AppBar(
          centerTitle: false,
          title: const MenuAppBarTitle(title: 'Kas'),
        ),
        body: const Center(
          child: EmptyState(
            icon: Icons.account_balance_wallet_outlined,
            title: 'Kas',
            description: 'Pengelolaan kas akan tersedia pada tahap berikutnya.',
          ),
        ),
      ),
    );
  }
}
