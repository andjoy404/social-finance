import 'package:flutter/material.dart';
import '../../../../core/widgets/empty_state.dart';

class CashScreen extends StatelessWidget {
  const CashScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Kas')),
      body: const Center(
        child: EmptyState(
          icon: Icons.account_balance_wallet_outlined,
          title: 'Kas',
          description: 'Pengelolaan kas akan tersedia pada tahap berikutnya.',
        ),
      ),
    );
  }
}
